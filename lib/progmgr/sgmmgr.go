package progmgr

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"msh/lib/config"
	"msh/lib/errco"
	"msh/lib/servctrl"
	"msh/lib/servstats"

	"github.com/shirou/gopsutil/mem"
)

var (
	// ReqSent communicates to main func that the first request is completed and msh can continue
	ReqSent chan bool = make(chan bool, 1)

	// API protocol version - needed by utils.go
	protv int = 2

	updAddr string = "https://api.github.com/repos/kinuseka/minecraft-server-hibernation/releases"

	// segment used for stats
	sgm *segment = &segment{
		m:      &sync.Mutex{},
		tk:     time.NewTicker(time.Second),
		defDur: 4 * time.Hour,
	}
)

// GitHub release structure
type GitHubRelease struct {
	TagName    string `json:"tag_name"`
	Name       string `json:"name"`
	Prerelease bool   `json:"prerelease"`
	Draft      bool   `json:"draft"`
}

type segment struct {
	m *sync.Mutex // segment mutex (initialized with sgm and not affected by reset function)

	tk        *time.Ticker  // segment ticker (every second)
	defDur    time.Duration // segment default duration
	startTime time.Time     // segment start time
	end       *time.Timer   // segment end timer

	// stats are reset when segment reset is invoked
	stats struct {
		preTerm  bool
		dur      int
		hibeDur  int
		usageCpu float64
		usageMem float64
		playSec  int
	}

	// push contains data for user notification
	push struct {
		tk       *time.Ticker // time ticker to send an update notification in chat
		verCheck string       // version check result
		messages []string     // the message shown by the notification
	}
}

// sgmMgr handles segment and all variables related
// [goroutine]
func sgmMgr() {
	// initialize sgm variables
	sgm.reset(0) // segment duration initialized to 0 so that the first request can be executed immediately

	// Signal main thread to continue immediately, don't wait for version check to complete
	select {
	case ReqSent <- true:
		errco.NewLogln(errco.TYPE_INF, errco.LVL_3, errco.ERROR_NIL, "Sent ready signal to main thread")
	default:
		errco.NewLogln(errco.TYPE_WAR, errco.LVL_3, errco.ERROR_NIL, "ReqSent channel already has a value")
	}

	for {
	mainselect:
		select {

		// segment 1 second tick
		case <-sgm.tk.C:
			sgm.m.Lock()

			// increment segment duration counter
			sgm.stats.dur += 1

			// increment hibernation duration counter if ms is not warm/interactable
			logMsh := servctrl.CheckMSWarm()
			if logMsh != nil {
				sgm.stats.hibeDur += 1
			}

			// increment play seconds sum
			sgm.stats.playSec += servstats.Stats.ConnCount

			// update segment average cpu/memory usage
			mshTreeCpu, mshTreeMem := getMshTreeStats()
			sgm.stats.usageCpu = (sgm.stats.usageCpu*float64(sgm.stats.dur-1) + float64(mshTreeCpu)) / float64(sgm.stats.dur) // sgm.stats.seconds-1 because the average is relative to 1 sec ago
			sgm.stats.usageMem = (sgm.stats.usageMem*float64(sgm.stats.dur-1) + float64(mshTreeMem)) / float64(sgm.stats.dur)

			if config.ConfigRuntime.Msh.ShowResourceUsage {
				memInfo, _ := mem.VirtualMemory()
				errco.NewLogln(errco.TYPE_INF, errco.LVL_3, errco.ERROR_NIL, "cpu avg: %7.3f %% cpu now: %7.3f %%  -  mem avg: %7.3f %% mem now: %7.3f %% (of %4d MB) = %7.3f MB",
					sgm.stats.usageCpu,
					mshTreeCpu,
					sgm.stats.usageMem,
					mshTreeMem,
					memInfo.Total/(1<<20),
					0.01*mshTreeMem*float64(memInfo.Total)/(1<<20),
				)
			}

			sgm.m.Unlock() // not using defer since it's an infinite loop

		// send a notification in game chat for players to see.
		// (should not send notification in console)
		case <-sgm.push.tk.C:
			if sgm.push.verCheck != "" && servstats.Stats.ConnCount > 0 {
				logMsh := servctrl.TellRaw("manager", sgm.push.verCheck, "sgmMgr")
				if logMsh != nil {
					logMsh.Log(true)
				}
			}

			if len(sgm.push.messages) != 0 && servstats.Stats.ConnCount > 0 {
				for _, m := range sgm.push.messages {
					logMsh := servctrl.TellRaw("message", m, "sgmMgr")
					if logMsh != nil {
						logMsh.Log(true)
					}
				}
			}

		// send request when segment ends
		case <-sgm.end.C:
			// Check version against GitHub
			errco.NewLogln(errco.TYPE_INF, errco.LVL_3, errco.ERROR_NIL, "Checking version against GitHub releases...")

			// Create HTTP client with timeout
			client := &http.Client{Timeout: 10 * time.Second}

			// Create request
			req, err := http.NewRequest("GET", updAddr, nil)
			if err != nil {
				errco.NewLogln(errco.TYPE_ERR, errco.LVL_3, errco.ERROR_VERSION, "Failed to create request: %s", err.Error())
				sgm.prolong(10 * time.Minute)
				break mainselect
			}

			// Add user agent header (GitHub API requires this)
			req.Header.Add("User-Agent", fmt.Sprintf("MSH/%s", MshVersion))

			// Send request
			res, err := client.Do(req)
			if err != nil {
				errco.NewLogln(errco.TYPE_ERR, errco.LVL_3, errco.ERROR_VERSION, "Failed to connect to GitHub API: %s", err.Error())
				errco.NewLogln(errco.TYPE_INF, errco.LVL_1, errco.ERROR_NIL, "Current version is probably a future one or couldn't check")
				sgm.prolong(10 * time.Minute)
				break mainselect
			}
			defer res.Body.Close()

			// Check response status code
			if res.StatusCode != 200 {
				body, _ := io.ReadAll(res.Body)
				errco.NewLogln(errco.TYPE_WAR, errco.LVL_3, errco.ERROR_VERSION, "GitHub API returned status %d: %s", res.StatusCode, string(body))
				errco.NewLogln(errco.TYPE_INF, errco.LVL_1, errco.ERROR_NIL, "Current version is probably a future one or couldn't check")
				sgm.prolong(10 * time.Minute)
				break mainselect
			}

			// Read and parse response
			var releases []GitHubRelease
			if err := json.NewDecoder(res.Body).Decode(&releases); err != nil {
				errco.NewLogln(errco.TYPE_ERR, errco.LVL_3, errco.ERROR_VERSION, "Failed to parse GitHub API response: %s", err.Error())
				sgm.prolong(10 * time.Minute)
				break mainselect
			}

			// Find latest stable release
			var latestRelease *GitHubRelease
			for _, release := range releases {
				// Skip drafts and prereleases
				if release.Draft || release.Prerelease {
					continue
				}

				latestRelease = &release
				break // First non-draft, non-prerelease is the latest stable
			}

			if latestRelease == nil {
				errco.NewLogln(errco.TYPE_WAR, errco.LVL_3, errco.ERROR_VERSION, "No stable releases found on GitHub")
				sgm.prolong(10 * time.Minute)
				// Continue execution instead of breaking
			} else {
				// Clean up version strings for comparison
				// Local version "v2.6.0" -> "2.6.0"
				localVersion := strings.TrimPrefix(MshVersion, "v")
				// GitHub version "v2.6.0" -> "2.6.0"
				latestVersion := strings.TrimPrefix(latestRelease.TagName, "v")

				errco.NewLogln(errco.TYPE_INF, errco.LVL_3, errco.ERROR_NIL, "Local version: %s, Latest GitHub version: %s", localVersion, latestVersion)

				// Compare versions using semantic versioning rules
				isOutdated, isNewer, err := compareVersions(localVersion, latestVersion)
				if err != nil {
					// Fallback to string comparison if semantic version parsing fails
					errco.NewLogln(errco.TYPE_WAR, errco.LVL_3, errco.ERROR_VERSION,
						"Failed to parse versions semantically: %s. Falling back to string comparison", err.Error())

					if localVersion < latestVersion {
						isOutdated = true
					} else if localVersion > latestVersion {
						isNewer = true
					}
				}

				if isOutdated {
					// Outdated version
					verCheck := fmt.Sprintf("msh (%s) is outdated. Latest version is %s", MshVersion, latestRelease.TagName)
					errco.NewLogln(errco.TYPE_WAR, errco.LVL_0, errco.ERROR_VERSION, verCheck)
					sgm.push.verCheck = verCheck
				} else if isNewer {
					// Future version
					verCheck := fmt.Sprintf("msh (%s) is newer than the latest GitHub release (%s)", MshVersion, latestRelease.TagName)
					errco.NewLogln(errco.TYPE_INF, errco.LVL_1, errco.ERROR_NIL, verCheck)
					sgm.push.verCheck = verCheck
				} else {
					// Current version
					verCheck := fmt.Sprintf("msh (%s) is up to date", MshVersion)
					errco.NewLogln(errco.TYPE_INF, errco.LVL_1, errco.ERROR_NIL, verCheck)
					sgm.push.verCheck = verCheck
				}
			}

			// Reset segment
			sgm.reset(4 * time.Hour) // Check again in 4 hours
		}
	}
}

// reset segment variables
// accepted parameters types: int, time.Duration, *http.Response
func (sgm *segment) reset(i interface{}) *segment {
	sgm.startTime = time.Now()
	switch v := i.(type) {
	case int:
		sgm.end = time.NewTimer(time.Duration(v) * time.Second)
	case time.Duration:
		sgm.end = time.NewTimer(v)
	case *http.Response:
		if xrr, err := strconv.Atoi(v.Header.Get("x-ratelimit-reset")); err == nil {
			sgm.end = time.NewTimer(time.Duration(xrr) * time.Second)
		} else {
			sgm.end = time.NewTimer(sgm.defDur)
		}
	default:
		sgm.end = time.NewTimer(sgm.defDur)
	}

	sgm.stats.dur = 0
	sgm.stats.hibeDur = 0
	sgm.stats.usageCpu, sgm.stats.usageMem = getMshTreeStats()
	sgm.stats.playSec = 0
	sgm.stats.preTerm = false

	sgm.push.tk = time.NewTicker(20 * time.Minute)
	sgm.push.verCheck = ""
	sgm.push.messages = []string{}

	return sgm
}

// prolong prolongs segment end timer. Should be called only when sgm.(*time.Timer).C has been drained
// accepted parameters types: int, time.Duration, *http.Response
func (sgm *segment) prolong(i interface{}) {
	sgm.m.Lock()
	defer sgm.m.Unlock()

	switch v := i.(type) {
	case int:
		sgm.end.Reset(time.Duration(v) * time.Second)
	case time.Duration:
		sgm.end = time.NewTimer(v)
	case *http.Response:
		if xrr, err := strconv.Atoi(v.Header.Get("x-ratelimit-reset")); err == nil {
			sgm.end.Reset(time.Duration(xrr) * time.Second)
		} else {
			sgm.end.Reset(sgm.defDur)
		}
	default:
		sgm.end.Reset(sgm.defDur)
	}
}

// compareVersions implements semantic versioning comparison
// Returns (isOutdated, isNewer, error)
func compareVersions(version1, version2 string) (bool, bool, error) {
	// Split version strings into components (major.minor.patch)
	v1Parts := strings.Split(version1, ".")
	v2Parts := strings.Split(version2, ".")

	// Check that we have at least major version component
	if len(v1Parts) == 0 || len(v2Parts) == 0 {
		return false, false, fmt.Errorf("invalid version format")
	}

	// Compare components one by one (major, minor, patch)
	for i := 0; i < 3; i++ {
		// Get components or default to 0 if not present
		var v1Comp, v2Comp int
		var err error

		if i < len(v1Parts) {
			v1Comp, err = strconv.Atoi(v1Parts[i])
			if err != nil {
				return false, false, fmt.Errorf("invalid version component %s: %v", v1Parts[i], err)
			}
		}

		if i < len(v2Parts) {
			v2Comp, err = strconv.Atoi(v2Parts[i])
			if err != nil {
				return false, false, fmt.Errorf("invalid version component %s: %v", v2Parts[i], err)
			}
		}

		// Compare numerically
		if v1Comp < v2Comp {
			return true, false, nil // version1 is outdated
		} else if v1Comp > v2Comp {
			return false, true, nil // version1 is newer
		}

		// If equal, continue to next component
	}

	// All components are equal
	return false, false, nil
}
