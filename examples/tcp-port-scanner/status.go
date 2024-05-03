package main

type Status uint8

const (
	Unknown Status = iota
	Open
	Closed
)

func (s Status) String() string {
	statuses := map[Status]string{
		Unknown: "Undefined",
		Open:    "Open",
		Closed:  "Closed",
	}
	return statuses[s]
}

func (s Status) MarshalJSON() ([]byte, error) {
	return []byte(`"` + s.String() + `"`), nil
}
