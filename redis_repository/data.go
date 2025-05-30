package redis_repository

type ExerciseContext struct {
	Exercises  []string `json:"exercises"`
	Attributes []string `json:"attributes"`
} 