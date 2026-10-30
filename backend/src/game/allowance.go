package game

import (
	"fmt"
	"strings"
	"sync"

	"sojourn/app"
)

const (
	ResourceWrites = "writes"
	ResourceReads  = "reads"
)

// chargedResources maps each metered verb to the allowance it draws from
// (charter R4.2/R4.3). Verbs not listed cost nothing.
var chargedResources = map[string]string{
	"POKE": ResourceWrites,
	"CALL": ResourceWrites,
	"TRIM": ResourceWrites,
	"SAFE": ResourceWrites,
	"PING": ResourceReads,
	"STAT": ResourceReads,
	"PEEK": ResourceReads,
	"DUMP": ResourceReads,
	"AUTH": ResourceReads,
}

func chargedResource(verb string) string {
	return chargedResources[verb]
}

// allowance meters admitted commands against a scenario's budget. Resources
// the scenario leaves out are unmetered.
type allowance struct {
	mu     sync.Mutex
	limits map[string]int
	used   map[string]int
}

func newAllowance(budget BudgetConfig) *allowance {
	a := &allowance{limits: map[string]int{}, used: map[string]int{}}
	if budget.Writes != nil {
		a.limits[ResourceWrites] = *budget.Writes
	}
	if budget.Reads != nil {
		a.limits[ResourceReads] = *budget.Reads
	}
	return a
}

// admit reserves one use of resource, reporting false if it is exhausted.
func (a *allowance) admit(resource string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	limit, metered := a.limits[resource]
	if !metered {
		return true
	}
	if a.used[resource] >= limit {
		return false
	}
	a.used[resource]++
	return true
}

func (a *allowance) remaining() map[string]int {
	remaining := map[string]int{}
	if a == nil {
		return remaining
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	for resource, limit := range a.limits {
		remaining[resource] = limit - a.used[resource]
	}
	return remaining
}

// admitUplink refuses commands whose allowance is exhausted.
func admitUplink(playerUplinkChan <-chan []byte, admittedUplinkChan chan<- []byte, allowance *allowance,
	refusals chan<- DownlinkMessage, refusalsDone <-chan struct{}) {
	defer close(admittedUplinkChan)

	for uplinkMessage := range playerUplinkChan {
		raw := strings.TrimSpace(string(uplinkMessage))
		resource := chargedResource(parseCommand(raw).Verb)

		if !allowance.admit(resource) {
			app.GameLogger.Printf("Refused `%s`: %s allowance exhausted", raw, resource)
			select {
			case refusals <- &CommandRefusedDownlinkMessage{
				Command:     raw,
				Reason:      fmt.Sprintf("%s allowance exhausted", resource),
				Constraints: allowance.remaining(),
			}:
			case <-refusalsDone:
			}
			continue
		}

		admittedUplinkChan <- uplinkMessage
	}
}

func budgetUsed(log []CommandLogEntry) map[string]int {
	used := map[string]int{ResourceWrites: 0, ResourceReads: 0}
	for _, command := range log {
		if command.Charged != "" {
			used[command.Charged]++
		}
	}
	return used
}
