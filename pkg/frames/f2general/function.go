package f2general

import frame2 "github.com/hash-d/frame2/pkg"

// Give it a function, and it will execute.  No options,
// as we can predict what would be needed.  If you need
// input, you'll have to capture it in a closure
type Function struct {
	Fn func() error

	frame2.Log
}

func (f Function) Execute() error {
	return f.Fn()
}

func (f Function) Validate() error {
	return f.Fn()
}
