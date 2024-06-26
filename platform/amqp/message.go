package payload

import "time"

type GradeRequestMessage struct {
	Language  string               `json:"language"`
	SourceUrl string               `json:"sourceUrl"`
	Settings  GradeSettingsMessage `json:"settings"`
	Test      []GradeTestMessage   `json:"test"`
	Metadata  GradeMetadataMessage `json:"metadata"`
}

type GradeSettingsMessage struct {
	TimeLimit         int  `json:"timeLimit"`
	MemoryLimit       int  `json:"memoryLimit"`
	IsAutoTrimEnabled bool `json:"isAutoTrimEnabled"`
}

type GradeTestMessage struct {
	InputUrl  string `json:"input"`
	OutputUrl string `json:"output"`
}

type GradeMetadataMessage struct {
	AssignmentId int       `json:"assignmentId"`
	SubmissionId int       `json:"submissionId"`
	TestcaseIds  []int     `json:"testcaseIds"`
	StartTime    time.Time `json:"startTime"`
}

type GradeResponseMessage struct {
	CompileOutput string                       `json:"compileOutput"`
	Status        string                       `json:"status"`
	Results       []GradeResponseResultMessage `json:"results"`
	Metadata      GradeMetadataMessage         `json:"metadata"`
}

type GradeResponseResultMessage struct {
	Hash   string `json:"hash"`
	Pass   bool   `json:"pass"`
	Time   int    `json:"time"`
	Memory int    `json:"memory"`
}
