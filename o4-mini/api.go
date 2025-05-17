package o4mini

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

func ProcessMessage(message string) ([]Exercise, error) {
	ctx := context.Background()
	llm, err := openai.New(openai.WithModel("gpt-4.1-nano"), openai.WithResponseFormat(&openai.ResponseFormat{Type: "json_object"}))
	if err != nil {
		log.Fatal(err)
	}
	prompt := `You are a workout analyzer AI that extracts and structures workout information from user messages.

TASK:
Parse the message below and identify all exercises mentioned. Return ONLY a JSON array of exercise objects that follow the schema specified below.

INPUT MESSAGE:
` + message + `

RESPONSE FORMAT:
Return ONLY a valid JSON object containing an array of exercise objects with the following structure:
{ 
	"response": [
		{
			"exercise_name": "Name of the exercise",
			"summary": "Brief description if available",
			"type": "strength, cardio, flexibility, etc.",
			"sets": number of sets if applicable,
			"work": numeric quantity of work (reps, distance, etc.),
			"work_type": "repetitions", "miles", "kilometers", etc.,
			"resistance": amount of resistance if applicable,
			"resistance_type": "pounds", "kilograms", "bodyweight", etc.,
			"duration": duration in minutes if applicable,
			"attributes": ["any", "relevant", "tags"],
			"created_ts": "current timestamp in ISO format"
		}
	]
} 

RULES:
1. Only include fields that are explicitly mentioned in the message
2. Return an empty array if no exercises are detected
3. Be precise about extracting the exact exercise names and details
4. The response must be ONLY the JSON array with no additional text or explanations
5. Use null for missing optional values, do not include empty strings
6. Make educated inferences only when the data strongly implies certain values
7. Always return an array of JSON objects, even if the array only contains a single item
8. Convert all numbers into their numeric articulation (thirty should be converted to 30)
9. Always standardize to full, plural spelling of a measurement (lb -> pounds), (sec->seconds)
`

	completion, err := llms.GenerateFromSinglePrompt(ctx, llm, prompt)
	log.Printf("Completed Prompt:%v", completion)
	if err != nil {
		return nil, fmt.Errorf("could not generate json from prompt %v", err)
	}
	var exercises Output
	if err := json.Unmarshal([]byte(completion), &exercises); err != nil {
		return nil, fmt.Errorf("could not marshal json %v", err)

	}
	return exercises.Response, nil
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

type Output struct {
	Response []Exercise `json:"response"`
}
