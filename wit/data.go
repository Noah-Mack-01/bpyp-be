package wit

import "time"

type WorkoutData struct {
	Entities map[string][]EntityValue `json:"entities"`
	Intents  []Intent                 `json:"intents"`
	Text     string                   `json:"text"`
	Traits   map[string][]Trait       `json:"traits"`
}

type EntityValue struct {
	Body       string                 `json:"body"`
	Confidence float64                `json:"confidence"`
	End        int                    `json:"end"`
	Entities   map[string]interface{} `json:"entities"`
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Role       string                 `json:"role"`
	Start      int                    `json:"start"`
	Suggested  bool                   `json:"suggested,omitempty"`
	Type       string                 `json:"type"`
	Value      interface{}            `json:"value"`
	Unit       string                 `json:"unit,omitempty"`
}

type Intent struct {
	Confidence float64 `json:"confidence"`
	ID         string  `json:"id"`
	Name       string  `json:"name"`
}

type Trait struct {
	Confidence float64     `json:"confidence"`
	ID         string      `json:"id"`
	Value      interface{} `json:"value"`
	Type       string      `json:"type"`
}

// Output exercise schema
type Exercise struct {
	Exercise       string    `json:"exercise_name"`
	Summary        string    `json:"summary,omitempty"`
	Type           string    `json:"type,omitempty"`
	Sets           float64   `json:"sets,omitempty"`
	Quantity       float64   `json:"work,omitempty"`
	QuantityType   string    `json:"work_type,omitempty"`
	Resistance     float64   `json:"resistance,omitempty"`
	ResistanceType string    `json:"resistance_type,omitempty"`
	Duration       float64   `json:"duration,omitempty"`
	Attributes     []string  `json:"attributes,omitempty"`
	UserId         string    `json:"user_id,omitempty"`
	Timestamp      time.Time `json:"created_ts"`
	Id             string    `json:"id,omitempty"`
}
