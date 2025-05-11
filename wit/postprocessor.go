package wit

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Input JSON structures

// ProcessWorkoutData extracts structured exercise data from workout data
func ProcessWorkoutData(data []byte) ([]Exercise, error) {
	var workoutData WorkoutData
	err := json.Unmarshal(data, &workoutData)
	if err != nil {
		return nil, fmt.Errorf("error parsing JSON: %w", err)
	}

	exercises := extractExercisesFromWorkoutData(workoutData)
	return exercises, nil
}

// Extract exercises from workout data
func extractExercisesFromWorkoutData(workoutData WorkoutData) []Exercise {
	exerciseMap := make(map[string]*Exercise)

	// Process exercise entities
	if exerciseEntities, ok := workoutData.Entities["exercise:exercise"]; ok {
		for _, entity := range exerciseEntities {
			exerciseName := cleanText(entity.Body)

			// Create new exercise entry if it doesn't exist
			if _, exists := exerciseMap[exerciseName]; !exists {
				exerciseMap[exerciseName] = &Exercise{
					Exercise:   exerciseName,
					Attributes: []string{},
				}
			}
		}
	}

	// If no exercise entities were found, return empty
	if len(exerciseMap) == 0 {
		return nil
	}

	// Get the first exercise as default for any modifiers without clear association
	var defaultExercise *Exercise
	for _, ex := range exerciseMap {
		defaultExercise = ex
		break
	}

	// Process sets
	if setsEntities, ok := workoutData.Entities["wit$number:implied_sets"]; ok {
		for _, entity := range setsEntities {
			// Extract sets value
			var setCount float64
			switch v := entity.Value.(type) {
			case float64:
				setCount = v
			case int:
				setCount = float64(v)
			case string:
				if num, err := strconv.ParseFloat(v, 64); err == nil {
					setCount = num
				} else {
					setCount = parseWordNumber(v)
				}
			}

			// Apply to closest exercise or default
			closestExercise := findClosestExercise(entity, workoutData, exerciseMap)
			if closestExercise != nil {
				closestExercise.Sets = setCount
			} else if defaultExercise != nil {
				defaultExercise.Sets = setCount
			}
		}
	}

	// Process reps
	if repEntities, ok := workoutData.Entities["wit$number:implied_reps"]; ok {
		for _, entity := range repEntities {
			// Extract reps value
			var repCount float64
			switch v := entity.Value.(type) {
			case float64:
				repCount = v
			case int:
				repCount = float64(v)
			case string:
				if num, err := strconv.ParseFloat(v, 64); err == nil {
					repCount = num
				} else {
					repCount = parseWordNumber(v)
				}
			}

			// Apply to closest exercise or default
			closestExercise := findClosestExercise(entity, workoutData, exerciseMap)
			if closestExercise != nil {
				closestExercise.Quantity = repCount
				closestExercise.QuantityType = "repetitions"
			} else if defaultExercise != nil {
				defaultExercise.Quantity = repCount
				defaultExercise.QuantityType = "repetitions"
			}
		}
	}

	// Process duration
	if durationEntities, ok := workoutData.Entities["wit$duration:duration"]; ok {
		for _, entity := range durationEntities {
			// Extract duration value
			var duration float64
			switch v := entity.Value.(type) {
			case float64:
				duration = v
			case int:
				duration = float64(v)
			case map[string]interface{}:
				if val, ok := v["value"]; ok {
					if numVal, ok := val.(float64); ok {
						duration = numVal
					}
				}
			}

			// Apply to closest exercise or default
			closestExercise := findClosestExercise(entity, workoutData, exerciseMap)
			if closestExercise != nil {
				closestExercise.Quantity = duration
				closestExercise.QuantityType = "duration"
			} else if defaultExercise != nil {
				defaultExercise.Quantity = duration
				defaultExercise.QuantityType = "duration"
			}
		}
	}

	// Process distance
	if distanceEntities, ok := workoutData.Entities["wit$distance:distance"]; ok {
		for _, entity := range distanceEntities {
			// Extract distance value
			var distance float64
			switch v := entity.Value.(type) {
			case float64:
				distance = v
			case int:
				distance = float64(v)
			case map[string]interface{}:
				if val, ok := v["value"]; ok {
					if numVal, ok := val.(float64); ok {
						distance = numVal
					}
				}
			}

			// Apply to closest exercise or default
			closestExercise := findClosestExercise(entity, workoutData, exerciseMap)
			if closestExercise != nil {
				closestExercise.Quantity = distance
				closestExercise.QuantityType = "distance"
			} else if defaultExercise != nil {
				defaultExercise.Quantity = distance
				defaultExercise.QuantityType = "distance"
			}
		}
	}

	// Process weight
	if weightEntities, ok := workoutData.Entities["wit$quantity:weight"]; ok {
		for _, entity := range weightEntities {
			// Extract weight value
			var weight float64
			switch v := entity.Value.(type) {
			case float64:
				weight = v
			case int:
				weight = float64(v)
			}

			// Get unit
			resistanceType := "pounds"
			if entity.Unit == "kilogram" {
				resistanceType = "kg"
			} else if entity.Unit == "pound" {
				resistanceType = "pounds"
			}

			// Apply to closest exercise or default
			closestExercise := findClosestExercise(entity, workoutData, exerciseMap)
			if closestExercise != nil {
				closestExercise.Resistance = weight
				closestExercise.ResistanceType = resistanceType
			} else if defaultExercise != nil {
				defaultExercise.Resistance = weight
				defaultExercise.ResistanceType = resistanceType
			}
		}
	}

	// Process attributes
	if attrEntities, ok := workoutData.Entities["exercise:attribute"]; ok {
		for _, entity := range attrEntities {
			attr := cleanText(entity.Body)

			// Apply to closest exercise or default
			closestExercise := findClosestExercise(entity, workoutData, exerciseMap)
			if closestExercise != nil {
				closestExercise.Attributes = append(closestExercise.Attributes, attr)
			} else if defaultExercise != nil {
				defaultExercise.Attributes = append(defaultExercise.Attributes, attr)
			}
		}
	}

	// Convert map to slice
	var result []Exercise
	for _, ex := range exerciseMap {
		result = append(result, *ex)
	}

	return result
}

// Find the closest exercise to an entity
func findClosestExercise(entity EntityValue, data WorkoutData, exerciseMap map[string]*Exercise) *Exercise {
	if len(exerciseMap) == 0 {
		return nil
	}

	if len(exerciseMap) == 1 {
		for _, ex := range exerciseMap {
			return ex
		}
	}

	// Find the closest exercise by text proximity
	entityMidpoint := (entity.Start + entity.End) / 2
	closestDistance := 1000000
	var closestExercise *Exercise

	for name, ex := range exerciseMap {
		// Try to find the entity in the original text
		exerciseEntities, ok := data.Entities["exercise:exercise"]
		if !ok {
			continue
		}

		for _, exEntity := range exerciseEntities {
			if cleanText(exEntity.Body) == name {
				exMidpoint := (exEntity.Start + exEntity.End) / 2
				distance := abs(entityMidpoint - exMidpoint)

				if distance < closestDistance {
					closestDistance = distance
					closestExercise = ex
				}
			}
		}
	}

	return closestExercise
}

// Parse number words like "five" or "twenty"
func parseWordNumber(text string) float64 {
	wordNumbers := map[string]float64{
		"one":       1,
		"two":       2,
		"three":     3,
		"four":      4,
		"five":      5,
		"six":       6,
		"seven":     7,
		"eight":     8,
		"nine":      9,
		"ten":       10,
		"eleven":    11,
		"twelve":    12,
		"thirteen":  13,
		"fourteen":  14,
		"fifteen":   15,
		"sixteen":   16,
		"seventeen": 17,
		"eighteen":  18,
		"nineteen":  19,
		"twenty":    20,
		"thirty":    30,
		"forty":     40,
		"fifty":     50,
		"sixty":     60,
		"seventy":   70,
		"eighty":    80,
		"ninety":    90,
		"hundred":   100,
	}

	lowerText := strings.ToLower(text)

	for word, value := range wordNumbers {
		if strings.Contains(lowerText, word) {
			return value
		}
	}

	return 0
}

// Clean text by removing unwanted characters
func cleanText(text string) string {
	// Trim spaces and punctuation
	text = strings.TrimSpace(text)
	text = strings.Trim(text, ".,;:!?-")
	return text
}

// Get absolute value
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// Example usage
func PostProcess(data []byte) ([]byte, []Exercise, error) {
	// Example of a simple workout data in the new format
	exercises, err := ProcessWorkoutData(data)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return nil, nil, err
	}

	// Output the processed exercises
	result, _ := json.MarshalIndent(exercises, "", "  ")
	fmt.Println(string(result))
	return result, exercises, nil
}
