package systemrun

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"software-studio/internal/model"
)

// Do not inherit the embedded delivery assertion's decoder: the system adds a
// channel identity. Keep explicit zero attempts distinct from missing/null.
func (e *MessageExpectation) UnmarshalJSON(data []byte) error {
	var wire struct {
		ChannelID string `json:"channelId"`
		Name      string `json:"name"`
		Status    string `json:"status"`
		Attempts  *int   `json:"attempts"`
		Accepted  *bool  `json:"accepted,omitempty"`
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("expected one message assertion")
	}
	if wire.Attempts == nil {
		return fmt.Errorf("message attempts is required and cannot be null")
	}
	*e = MessageExpectation{ChannelID: wire.ChannelID, DeliveryMessageExpectation: model.DeliveryMessageExpectation{Name: wire.Name, Status: wire.Status, Attempts: *wire.Attempts, Accepted: wire.Accepted}}
	return nil
}
