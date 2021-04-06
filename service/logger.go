package service

import (
	"fmt"
	"os"
)

type Logger interface {
	Infof(format string, a ...interface{})
	Errorf(format string, a ...interface{})
}

type EmptyLogger struct{}

func (o *EmptyLogger) Infof(format string, a ...interface{})  {}
func (o *EmptyLogger) Errorf(format string, a ...interface{}) {}

type StdOutLogger struct{}

func (o *StdOutLogger) Infof(format string, a ...interface{}) {
	_, _ = fmt.Fprintf(os.Stdout, format+"\n", a...)
}
func (o *StdOutLogger) Errorf(format string, a ...interface{}) {
	_, _ = fmt.Fprintf(os.Stdout, format+"\n", a...)
}
