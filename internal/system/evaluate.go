package system

import "fmt"

type requestEvidence struct {
	received, terminal bool
	records            map[string]bool
	success            []Event
	persisted          map[string]int
	sequences          []int
}

// Evaluate checks only the bounded observation supplied by the caller. A trace
// adapter is responsible for reporting loss as evidence_gap. Claims cannot
// establish completeness of an untrusted observer or prove future behavior.
func Evaluate(c Contract, events []Event) []Verdict {
	verdicts := make([]Verdict, 0, len(c.Claims))
	requests := map[string]*requestEvidence{}
	order := []string{}
	gap := len(events) == 0
	previous := -1
	for _, e := range events {
		if e.Sequence <= previous || e.Sequence < 0 {
			gap = true
		}
		previous = e.Sequence
		if e.Kind == "evidence_gap" {
			gap = true
			continue
		}
		switch e.Kind {
		case "request_received", "persisted", "response_success", "response_failure":
		default:
			gap = true
			continue
		}
		if e.RequestID == "" {
			gap = true
			continue
		}
		r := requests[e.RequestID]
		if r == nil {
			r = &requestEvidence{records: map[string]bool{}, persisted: map[string]int{}}
			requests[e.RequestID] = r
			order = append(order, e.RequestID)
		}
		r.sequences = append(r.sequences, e.Sequence)
		if e.Kind != "request_received" && !r.received {
			gap = true
		}
		switch e.Kind {
		case "request_received":
			r.received = true
		case "persisted":
			if e.EncounterID == "" {
				gap = true
			} else {
				r.records[e.EncounterID] = true
				if _, ok := r.persisted[e.EncounterID]; !ok {
					r.persisted[e.EncounterID] = e.Sequence
				}
			}
		case "response_success":
			r.terminal = true
			r.success = append(r.success, e)
			if e.EncounterID == "" {
				gap = true
			}
		case "response_failure":
			r.terminal = true
		}
	}
	for _, r := range requests {
		if !r.received || !r.terminal {
			gap = true
		}
	}
	invalid := len(ValidateContract(c)) > 0
	for _, claim := range c.Claims {
		v := Verdict{ClaimID: claim.ID, Status: "satisfied", Message: "Claim satisfied for this bounded run under the declared assumptions.", EvidenceSequences: []int{}}
		if invalid {
			v.Status = "inconclusive"
			v.Message = "The contract is invalid; validate it before evaluating evidence."
			verdicts = append(verdicts, v)
			continue
		}
		if gap {
			v.Status = "inconclusive"
			v.Message = "Evidence is missing, malformed, incomplete, or explicitly marked as lost."
		}
		for _, id := range order {
			r := requests[id]
			violation := ""
			switch claim.Kind {
			case "exactly_one_record":
				if len(r.records) > 1 {
					violation = fmt.Sprintf("Request %q persisted %d distinct encounters.", id, len(r.records))
				} else if !gap && len(r.success) > 0 && len(r.records) != 1 {
					violation = fmt.Sprintf("Successful request %q has no persisted encounter.", id)
				}
			case "response_matches_record", "no_false_success":
				if !gap {
					for _, e := range r.success {
						seq, ok := r.persisted[e.EncounterID]
						if !ok || seq >= e.Sequence {
							violation = fmt.Sprintf("Request %q reported encounter %q before matching persistence evidence.", id, e.EncounterID)
							break
						}
					}
				}
			case "required_evidence":
				// Completeness is checked above. Missing observations are unknown,
				// never evidence that the application itself violated its behavior.
			}
			if violation != "" {
				v.Status = "violated"
				v.Message = violation
				v.EvidenceSequences = append(v.EvidenceSequences, r.sequences...)
				break
			}
			v.EvidenceSequences = append(v.EvidenceSequences, r.sequences...)
		}
		verdicts = append(verdicts, v)
	}
	return verdicts
}
