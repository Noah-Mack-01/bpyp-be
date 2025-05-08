package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// WitResponse represents the structure of the Wit.ai response
type WitResponse struct {
	Text     string                       `json:"text"`
	Intents  []Intent                     `json:"intents"`
	Entities map[string][]EntityContainer `json:"entities"`
	Traits   map[string][]Trait           `json:"traits"`
}

type Intent struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Confidence float64 `json:"confidence"`
}

type EntityContainer struct {
	ID         string                 `json:"id"`
	Name       string                 `json:"name"`
	Role       string                 `json:"role"`
	Start      int                    `json:"start"`
	End        int                    `json:"end"`
	Body       string                 `json:"body"`
	Confidence float64                `json:"confidence"`
	Entities   map[string]interface{} `json:"entities"`
	Type       string                 `json:"type"`
	Value      interface{}            `json:"value"`
	Unit       string                 `json:"unit,omitempty"`
}

type Trait struct {
	ID         string  `json:"id"`
	Value      string  `json:"value"`
	Confidence float64 `json:"confidence"`
}

// Exercise represents a workout exercise
type Exercise struct {
	Name       string   `json:"name"`
	Reps       int      `json:"reps,omitempty"`
	Sets       int      `json:"sets,omitempty"`
	Weight     float64  `json:"weight,omitempty"`
	WeightUnit string   `json:"weight_unit,omitempty"`
	Distance   float64  `json:"distance,omitempty"`
	DistUnit   string   `json:"distance_unit,omitempty"`
	Duration   float64  `json:"duration,omitempty"`
	DurUnit    string   `json:"duration_unit,omitempty"`
	Attributes []string `json:"attributes,omitempty"`
}

// Workout represents a complete workout
type Workout struct {
	Exercises []Exercise `json:"exercises"`
	Duration  float64    `json:"total_duration,omitempty"`
	DurUnit   string     `json:"duration_unit,omitempty"`
	Distance  float64    `json:"total_distance,omitempty"`
	DistUnit  string     `json:"distance_unit,omitempty"`
}

// ProcessWorkout processes a WitResponse and returns a structured Workout
func ProcessWorkout(witResp WitResponse) Workout {
	workout := Workout{
		Exercises: []Exercise{},
	}

	// Extract all entities and sort them by position
	var allEntities []EntityContainer
	for entityType, entities := range witResp.Entities {
		for _, entity := range entities {
			entity.Name = entityType // Ensure name is set
			allEntities = append(allEntities, entity)
		}
	}

	// Sort entities by start position
	sortEntitiesByPosition(allEntities)

	// Group entities by exercise
	exerciseGroups := groupEntitiesByExercise(allEntities)

	// Process each exercise group
	for _, group := range exerciseGroups {
		exercises := createExercisesFromGroup(group)
		workout.Exercises = append(workout.Exercises, exercises...)
	}

	// Extract workout-level metrics that aren't tied to a specific exercise
	workout = extractWorkoutLevelMetrics(workout, allEntities)

	return workout
}

// sortEntitiesByPosition sorts entities by their starting position
func sortEntitiesByPosition(entities []EntityContainer) {
	// Implemented using a simple bubble sort for clarity
	n := len(entities)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if entities[j].Start > entities[j+1].Start {
				entities[j], entities[j+1] = entities[j+1], entities[j]
			}
		}
	}
}

// Entity group represents entities that are close to each other and likely related to the same exercise
type EntityGroup struct {
	Exercises  []EntityContainer
	Sets       []EntityContainer
	Reps       []EntityContainer
	Weights    []EntityContainer
	Distances  []EntityContainer
	Durations  []EntityContainer
	Attributes []EntityContainer
}

// groupEntitiesByExercise groups entities that are likely related to the same exercise
// groupEntitiesByExercise groups entities that are likely related to the same exercise
func groupEntitiesByExercise(entities []EntityContainer) []EntityGroup {
	var groups []EntityGroup

	// First, identify all exercise entities
	var exerciseEntities []EntityContainer
	for _, entity := range entities {
		if strings.HasSuffix(entity.Name, "exercise") {
			exerciseEntities = append(exerciseEntities, entity)
		}
	}

	// If no exercises found, put everything in one group
	if len(exerciseEntities) == 0 {
		var group EntityGroup
		for _, entity := range entities {
			entityType := strings.Split(entity.Name, ":")[0]
			entityRole := ""
			if len(strings.Split(entity.Name, ":")) > 1 {
				entityRole = strings.Split(entity.Name, ":")[1]
			}

			switch {
			case entityRole == "attribute":
				group.Attributes = append(group.Attributes, entity)
			case entityType == "wit$number" && entityRole == "implied_sets" || entityType == "wit$quantity" && entityRole == "sets":
				group.Sets = append(group.Sets, entity)
			case entityType == "wit$number" && entityRole == "implied_reps" || entityType == "wit$quantity" && entityRole == "reps":
				group.Reps = append(group.Reps, entity)
			case entityType == "wit$quantity" && entityRole == "weight":
				group.Weights = append(group.Weights, entity)
			case entityType == "wit$distance" && entityRole == "distance":
				group.Distances = append(group.Distances, entity)
			case entityType == "wit$duration" && entityRole == "duration":
				group.Durations = append(group.Durations, entity)
			}
		}
		groups = append(groups, group)
		return groups
	}

	// Create one group for each exercise
	for _, exerciseEntity := range exerciseEntities {
		group := EntityGroup{
			Exercises: []EntityContainer{exerciseEntity},
		}
		groups = append(groups, group)
	}

	// Now distribute other entities to the closest exercise group
	for _, entity := range entities {
		if strings.HasSuffix(entity.Name, "exercise") {
			continue // Skip exercise entities as they're already in groups
		}

		entityType := strings.Split(entity.Name, ":")[0]
		entityRole := ""
		if len(strings.Split(entity.Name, ":")) > 1 {
			entityRole = strings.Split(entity.Name, ":")[1]
		}

		// Find the closest exercise to this entity
		closestIdx := findClosestExerciseGroup(groups, entity)
		if closestIdx >= 0 {
			// Add entity to the appropriate collection in the closest group
			switch {
			case entityRole == "attribute":
				groups[closestIdx].Attributes = append(groups[closestIdx].Attributes, entity)
			case entityType == "wit$number" && entityRole == "implied_sets" || entityType == "wit$quantity" && entityRole == "sets":
				groups[closestIdx].Sets = append(groups[closestIdx].Sets, entity)
			case entityType == "wit$number" && entityRole == "implied_reps" || entityType == "wit$quantity" && entityRole == "reps":
				groups[closestIdx].Reps = append(groups[closestIdx].Reps, entity)
			case entityType == "wit$quantity" && entityRole == "weight":
				groups[closestIdx].Weights = append(groups[closestIdx].Weights, entity)
			case entityType == "wit$distance" && entityRole == "distance":
				groups[closestIdx].Distances = append(groups[closestIdx].Distances, entity)
			case entityType == "wit$duration" && entityRole == "duration":
				groups[closestIdx].Durations = append(groups[closestIdx].Durations, entity)
			}
		}
	}

	// Handle the case of global sets/circuits that apply to multiple exercise groups
	groups = handleGlobalSets(entities, groups)

	return groups
}

// findClosestExerciseGroup finds the index of the exercise group closest to the given entity
func findClosestExerciseGroup(groups []EntityGroup, entity EntityContainer) int {
	if len(groups) == 0 {
		return -1
	}

	closestIdx := 0

	// Reps typically appear BEFORE their exercise in natural language
	if strings.Contains(entity.Name, "reps") {
		// For repetitions, prefer exercises that come AFTER the rep count
		var minAfterDistance = math.MaxInt32
		var afterIdx = -1

		for i, group := range groups {
			if len(group.Exercises) == 0 {
				continue
			}

			ex := group.Exercises[0]
			if ex.Start > entity.End { // Exercise starts after rep entity ends
				distance := ex.Start - entity.End
				if distance < minAfterDistance {
					minAfterDistance = distance
					afterIdx = i
				}
			}
		}

		if afterIdx >= 0 {
			return afterIdx
		}
	}

	// For other types of entities, or if we didn't find an exercise after the rep entity,
	// find the closest exercise by absolute distance
	minDistance := calculateDistance(groups[0].Exercises[0], entity)

	for i, group := range groups {
		if len(group.Exercises) == 0 {
			continue
		}

		distance := calculateDistance(group.Exercises[0], entity)
		if distance < minDistance {
			minDistance = distance
			closestIdx = i
		}
	}

	return closestIdx
}

// calculateDistance calculates the "distance" between two entities in the text
func calculateDistance(e1, e2 EntityContainer) int {
	// If entities overlap, distance is 0
	if (e1.Start <= e2.Start && e1.End >= e2.Start) ||
		(e2.Start <= e1.Start && e2.End >= e1.Start) {
		return 0
	}

	// Otherwise, calculate the distance between the closest edges
	if e1.End < e2.Start {
		return e2.Start - e1.End
	}
	return e1.Start - e2.End
}

// handleGlobalSets processes set values that apply to multiple exercises
func handleGlobalSets(allEntities []EntityContainer, groups []EntityGroup) []EntityGroup {
	// Look for implied sets that appear before multiple exercises
	for _, entity := range allEntities {
		if strings.Contains(entity.Name, "implied_sets") || strings.Contains(entity.Name, "sets") {
			// Check if this set appears before a group of exercises but doesn't have a clear
			// association with just one exercise
			setPosition := entity.Start

			// Count how many exercise groups appear after this set
			affectedGroups := 0
			for i, group := range groups {
				// If the group doesn't have any sets and all exercises start after this set
				if len(group.Sets) == 0 && len(group.Exercises) > 0 && group.Exercises[0].Start > setPosition {
					groups[i].Sets = append(groups[i].Sets, entity)
					affectedGroups++
				}
			}

			// If this set affected multiple groups, we consider it a "global" set
			// and apply it to all exercise groups that didn't have explicit sets
			if affectedGroups > 1 {
				for i, group := range groups {
					if len(group.Sets) == 0 {
						groups[i].Sets = append(groups[i].Sets, entity)
					}
				}
			}
		}
	}

	return groups
}

// createExercisesFromGroup creates Exercise objects from an EntityGroup
func createExercisesFromGroup(group EntityGroup) []Exercise {
	var exercises []Exercise

	// If there are no exercises but other entities exist, create a default exercise
	if len(group.Exercises) == 0 {
		if len(group.Sets) > 0 || len(group.Reps) > 0 || len(group.Weights) > 0 ||
			len(group.Distances) > 0 || len(group.Durations) > 0 {
			exercise := Exercise{
				Name: "Unknown Exercise",
			}
			exercises = append(exercises, exercise)
		}
		return exercises
	}

	// Create an exercise for each exercise entity
	for _, exerciseEntity := range group.Exercises {
		exercise := Exercise{
			Name: cleanExerciseName(exerciseEntity.Body),
		}

		// Add attributes if available
		for _, attrEntity := range group.Attributes {
			if attrEntity.Start < exerciseEntity.End+15 && attrEntity.End > exerciseEntity.Start-15 {
				exercise.Attributes = append(exercise.Attributes, attrEntity.Body)
			}
		}

		exercises = append(exercises, exercise)
	}

	// Apply sets to all exercises in the group (sets typically apply to all exercises in a circuit)
	sets := extractNumberValue(group.Sets)
	if sets > 0 {
		for i := range exercises {
			exercises[i].Sets = sets
		}
	}

	// Apply reps, but match them to the closest exercise instead of applying the same to all
	for _, repEntity := range group.Reps {
		reps := extractNumberValueFromEntity(repEntity)
		if reps > 0 {
			// Find the closest exercise to this rep entity
			closestIdx := findClosestExercise(group.Exercises, repEntity)
			if closestIdx >= 0 && closestIdx < len(exercises) {
				exercises[closestIdx].Reps = reps
			}
		}
	}

	// For other metrics, we try to apply them to the closest exercise
	for _, weightEntity := range group.Weights {
		weight, unit := extractMeasurement(weightEntity)
		closestIdx := findClosestExercise(group.Exercises, weightEntity)
		if closestIdx >= 0 && closestIdx < len(exercises) {
			exercises[closestIdx].Weight = weight
			exercises[closestIdx].WeightUnit = unit
		}
	}

	for _, distEntity := range group.Distances {
		distance, unit := extractMeasurement(distEntity)
		closestIdx := findClosestExercise(group.Exercises, distEntity)
		if closestIdx >= 0 && closestIdx < len(exercises) {
			exercises[closestIdx].Distance = distance
			exercises[closestIdx].DistUnit = unit
		}
	}

	for _, durEntity := range group.Durations {
		duration, unit := extractMeasurement(durEntity)
		closestIdx := findClosestExercise(group.Exercises, durEntity)
		if closestIdx >= 0 && closestIdx < len(exercises) {
			exercises[closestIdx].Duration = duration
			exercises[closestIdx].DurUnit = unit
		}
	}

	return exercises
}

func extractNumberValueFromEntity(entity EntityContainer) int {
	// Try to get the value directly
	if val, ok := entity.Value.(float64); ok {
		return int(val)
	}

	// Otherwise, try to parse from the body
	body := entity.Body
	// Extract digits
	re := regexp.MustCompile(`\d+`)
	matches := re.FindString(body)
	if matches != "" {
		if val, err := strconv.Atoi(matches); err == nil {
			return val
		}
	}

	// Handle word numbers
	wordNumbers := map[string]int{
		"one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
		"six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10,
		"eleven": 11, "twelve": 12, "thirteen": 13, "fourteen": 14, "fifteen": 15,
		"sixteen": 16, "seventeen": 17, "eighteen": 18, "nineteen": 19, "twenty": 20,
		"thirty": 30, "forty": 40, "fifty": 50, "sixty": 60, "seventy": 70,
		"eighty": 80, "ninety": 90, "hundred": 100,
	}

	lowerBody := strings.ToLower(body)
	for word, val := range wordNumbers {
		if strings.Contains(lowerBody, word) {
			return val
		}
	}

	return 0
}

// findClosestExercise finds the index of the exercise entity closest to the given entity
func findClosestExercise(exercises []EntityContainer, entity EntityContainer) int {
	if len(exercises) == 0 {
		return -1
	}

	closestIdx := 0
	minDistance := abs(exercises[0].Start - entity.Start)

	for i, ex := range exercises {
		distance := abs(ex.Start - entity.Start)
		if distance < minDistance {
			minDistance = distance
			closestIdx = i
		}
	}

	return closestIdx
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// extractWorkoutLevelMetrics extracts metrics that apply to the entire workout
func extractWorkoutLevelMetrics(workout Workout, entities []EntityContainer) Workout {
	// Look for duration and distance entities that don't seem to be associated with a specific exercise
	for _, entity := range entities {
		if strings.Contains(entity.Name, "duration") {
			// Check if this duration isn't already assigned to an exercise
			alreadyAssigned := false
			for _, ex := range workout.Exercises {
				if ex.Duration > 0 {
					alreadyAssigned = true
					break
				}
			}

			if !alreadyAssigned {
				value, unit := extractMeasurement(entity)
				workout.Duration = value
				workout.DurUnit = unit
			}
		} else if strings.Contains(entity.Name, "distance") {
			// Check if this distance isn't already assigned to an exercise
			alreadyAssigned := false
			for _, ex := range workout.Exercises {
				if ex.Distance > 0 {
					alreadyAssigned = true
					break
				}
			}

			if !alreadyAssigned {
				value, unit := extractMeasurement(entity)
				workout.Distance = value
				workout.DistUnit = unit
			}
		}
	}

	return workout
}

// cleanExerciseName cleans up the exercise name
func cleanExerciseName(name string) string {
	// Remove numbers and common prefixes/suffixes
	re := regexp.MustCompile(`^\d+\s*|^\s*|\s*$`)
	cleaned := re.ReplaceAllString(name, "")

	// Further cleanup can be added here

	return cleaned
}

// extractNumberValue extracts a numeric value from entity containers
func extractNumberValue(entities []EntityContainer) int {
	if len(entities) == 0 {
		return 0
	}

	// Try to get the value directly
	if val, ok := entities[0].Value.(float64); ok {
		return int(val)
	}

	// Otherwise, try to parse from the body
	body := entities[0].Body
	// Extract digits
	re := regexp.MustCompile(`\d+`)
	matches := re.FindString(body)
	if matches != "" {
		if val, err := strconv.Atoi(matches); err == nil {
			return val
		}
	}

	// Handle word numbers
	wordNumbers := map[string]int{
		"one": 1, "two": 2, "three": 3, "four": 4, "five": 5,
		"six": 6, "seven": 7, "eight": 8, "nine": 9, "ten": 10,
		"eleven": 11, "twelve": 12, "thirteen": 13, "fourteen": 14, "fifteen": 15,
		"sixteen": 16, "seventeen": 17, "eighteen": 18, "nineteen": 19, "twenty": 20,
		"thirty": 30, "forty": 40, "fifty": 50, "sixty": 60, "seventy": 70,
		"eighty": 80, "ninety": 90, "hundred": 100, "thousand": 1000, "million": 1000000,
	}

	for word, val := range wordNumbers {
		if strings.Contains(strings.ToLower(body), word) {
			return val
		}
	}

	return 0
}

// extractMeasurement extracts a measurement value and unit from an entity
func extractMeasurement(entity EntityContainer) (float64, string) {
	// Try to get the value directly
	if val, ok := entity.Value.(float64); ok {
		return val, entity.Unit
	}

	// Otherwise, try to parse from the body
	body := entity.Body

	// Extract digits
	re := regexp.MustCompile(`\d+(\.\d+)?`)
	matches := re.FindString(body)
	if matches != "" {
		if val, err := strconv.ParseFloat(matches, 64); err == nil {
			// Try to determine the unit
			unitRe := regexp.MustCompile(`(lb|lbs|kg|kilos?|pounds?|minutes?|mins?|hrs?|hours?|seconds?|secs?|meters?|miles?|km|kilometers?)`)
			unitMatches := unitRe.FindString(strings.ToLower(body))

			var unit string
			switch {
			case strings.HasPrefix(unitMatches, "lb") || strings.HasPrefix(unitMatches, "pound"):
				unit = "lb"
			case strings.HasPrefix(unitMatches, "kg") || strings.HasPrefix(unitMatches, "kilo"):
				unit = "kg"
			case strings.HasPrefix(unitMatches, "min") || strings.HasPrefix(unitMatches, "minute"):
				unit = "min"
			case strings.HasPrefix(unitMatches, "hr") || strings.HasPrefix(unitMatches, "hour"):
				unit = "hour"
			case strings.HasPrefix(unitMatches, "sec") || strings.HasPrefix(unitMatches, "second"):
				unit = "sec"
			case strings.HasPrefix(unitMatches, "meter"):
				unit = "meter"
			case strings.HasPrefix(unitMatches, "mile"):
				unit = "mile"
			case strings.HasPrefix(unitMatches, "km") || strings.HasPrefix(unitMatches, "kilometer"):
				unit = "km"
			default:
				unit = ""
			}

			return val, unit
		}
	}

	return 0, ""
}

func main() {
	// Example Wit.ai response
	witRespJSON := `{
		"entities": {
			"exercise:exercise": [
				{
					"body": "burpees",
					"confidence": 0.9751,
					"end": 18,
					"entities": {},
					"id": "935049405286534",
					"name": "exercise",
					"role": "exercise",
					"start": 11,
					"suggested": true,
					"type": "value",
					"value": "burpees"
				},
				{
					"body": "jumping jacks",
					"confidence": 0.9862,
					"end": 35,
					"entities": {},
					"id": "935049405286534",
					"name": "exercise",
					"role": "exercise",
					"start": 23,
					"suggested": true,
					"type": "value",
					"value": "jumping jacks"
				}
			],
			"wit$number:implied_sets": [
				{
					"body": "2",
					"confidence": 1,
					"end": 1,
					"entities": {},
					"id": "666899662866322",
					"name": "wit$number",
					"role": "implied_sets",
					"start": 0,
					"type": "value",
					"value": 2
				}
			],
			"wit$number:implied_reps": [
				{
					"body": "twelve",
					"confidence": 1,
					"end": 10,
					"entities": {},
					"id": "666899662866322",
					"name": "wit$number",
					"role": "implied_reps",
					"start": 4,
					"type": "value",
					"value": 12
				},
				{
					"body": "ten",
					"confidence": 1,
					"end": 22,
					"entities": {},
					"id": "666899662866322",
					"name": "wit$number",
					"role": "implied_reps",
					"start": 19,
					"type": "value",
					"value": 10
				}
			]
		},
		"intents": [
			{
				"confidence": 0.9999,
				"id": "1406444540541447",
				"name": "record_workout"
			}
		],
		"text": "2 sets of twelve burpees, ten jumping jacks",
		"traits": {}
	}`

	var witResp WitResponse
	err := json.Unmarshal([]byte(witRespJSON), &witResp)
	if err != nil {
		log.Fatalf("Failed to unmarshal Wit.ai response: %v", err)
	}

	// Process the workout
	workout := ProcessWorkout(witResp)

	// Print the processed workout
	workoutJSON, err := json.MarshalIndent(workout, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal workout: %v", err)
	}
	fmt.Println(string(workoutJSON))
}
