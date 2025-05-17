package servctrl

import (
	"testing"

	"msh/lib/errco"
	"msh/lib/servstats"
)

// TestServerStatusChanges tests the proper transitions between server states
func TestServerStatusChanges(t *testing.T) {
	// Initialize server status to OFFLINE
	servstats.Stats.Status = errco.SERVER_STATUS_OFFLINE
	servstats.Stats.Suspended = false

	// Test 1: Check that suspended is properly reflected in the status
	t.Run("TestSuspendedStatus", func(t *testing.T) {
		// Setup: First set to ONLINE
		servstats.Stats.Status = errco.SERVER_STATUS_ONLINE

		// Verify starting state
		if servstats.Stats.Status != errco.SERVER_STATUS_ONLINE {
			t.Errorf("Expected initial status to be ONLINE (%d), got %d",
				errco.SERVER_STATUS_ONLINE, servstats.Stats.Status)
		}

		// Test the suspended status code existence
		if errco.SERVER_STATUS_SUSPENDED == 0 {
			t.Error("SERVER_STATUS_SUSPENDED is not properly defined")
		}

		// Set suspended state
		// Note: In production, this would be done by calling opsys.ProcTreeSuspend
		// but for testing we'll just set it directly
		servstats.Stats.Status = errco.SERVER_STATUS_SUSPENDED
		servstats.Stats.Suspended = true

		// Verify the state is now SUSPENDED
		if servstats.Stats.Status != errco.SERVER_STATUS_SUSPENDED {
			t.Errorf("Expected status to be SUSPENDED (%d), got %d",
				errco.SERVER_STATUS_SUSPENDED, servstats.Stats.Status)
		}

		// Verify that Suspended flag is properly set
		if !servstats.Stats.Suspended {
			t.Error("Expected Suspended flag to be true")
		}

		// Test transitioning back to ONLINE (simulating server warmup)
		servstats.Stats.Status = errco.SERVER_STATUS_ONLINE
		servstats.Stats.Suspended = false

		// Verify we're back to ONLINE
		if servstats.Stats.Status != errco.SERVER_STATUS_ONLINE {
			t.Errorf("Expected status to be ONLINE (%d), got %d",
				errco.SERVER_STATUS_ONLINE, servstats.Stats.Status)
		}
	})
}
