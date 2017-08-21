/*
Package go-sh is intented to make shell call with golang more easily.
Some usage is more similar to os/exec, eg: Run(), Output(), Command(name, args...)

But with these similar function, pipe is added in and this package also got shell-session support.

Why I love golang so much, because the usage of golang is simple, but the power is unlimited. I want to make this pakcage got the sample style like golang.

	// just like os/exec
	sh.Command("echo", "hello").Run()

	// support pipe
	sh.Command("echo", "hello").Command("wc", "-c").Run()

	// create a session to store dir and env
	sh.NewSession().SetDir("/").Command("pwd")

	// shell buildin command - "test"
	sh.Test("dir", "mydir")

	// like shell call: (cd /; pwd)
	sh.Command("pwd", sh.Dir("/")) same with sh.Command(sh.Dir("/"), "pwd")

	// output to json and xml easily
	v := map[string] int {}
	err = sh.Command("echo", `{"number": 1}`).UnmarshalJSON(&v)
*/
package sh

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/codegangsta/inject"
)

type Dir string

type Session struct {
	inj     inject.Injector
	alias   map[string][]string
	cmds    []*exec.Cmd
	dir     Dir
	started bool
	Env     map[string]string
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
	ShowCMD bool // enable for debug
	timeout time.Duration
}

func (s *Session) writePrompt(args ...interface{}) {
	var ps1 = fmt.Sprintf("[golang-sh]$")
	args = append([]interface{}{ps1}, args...)
	fmt.Fprintln(s.Stderr, args...)
}

func NewSession() *Session {
	env := make(map[string]string)
	for _, key := range []string{"PATH"} {
		env[key] = os.Getenv(key)
	}
	s := &Session{
		inj:    inject.New(),
		alias:  make(map[string][]string),
		dir:    Dir(""),
		Stdin:  strings.NewReader(""),
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Env:    env,
	}
	return s
}

func InteractiveSession() *Session {
	s := NewSession()
	s.SetStdin(os.Stdin)
	return s
}

func Command(name string, a ...interface{}) *Session {
	s := NewSession()
	return s.Command(name, a...)
}

func Echo(in string) *Session {
	s := NewSession()
	return s.SetInput(in)
}

func (s *Session) Alias(alias, cmd string, args ...string) {
	v := []string{cmd}
	v = append(v, args...)
	s.alias[alias] = v
}

func value2string(any interface{}) (string, bool) {
	anyt := reflect.TypeOf(any)
	anyv := reflect.ValueOf(any)
	switch anyt.Kind() {
	case reflect.Bool:
		return base2string(anyv.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return base2string(anyv.Int())
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return base2string(anyv.Uint())
	case reflect.Float32, reflect.Float64:
		return base2string(anyv.Float())
	case reflect.Complex64, reflect.Complex128:
		return base2string(anyv.Complex())
	case reflect.String:
		return anyv.String(), true
	default:
		return "", false
	}
}

func base2string(any interface{}) (value string, success bool) {
	value = ""
	success = true
	switch anyi := any.(type) {
	case string:
		value = any.(string)
	case bool:
		value = strconv.FormatBool(any.(bool))
	case int:
		value = strconv.FormatInt(int64(any.(int)), 10)
	case int8:
		value = strconv.FormatInt(int64(any.(int8)), 10)
	case int16:
		value = strconv.FormatInt(int64(any.(int16)), 10)
	case int32:
		value = strconv.FormatInt(int64(any.(int32)), 10)
	case int64:
		value = strconv.FormatInt(int64(any.(int64)), 10)
	case uint:
		value = strconv.FormatUint(uint64(any.(uint)), 10)
	case uint8:
		value = strconv.FormatUint(uint64(any.(uint8)), 10)
	case uint16:
		value = strconv.FormatUint(uint64(any.(uint16)), 10)
	case uint32:
		value = strconv.FormatUint(uint64(any.(uint32)), 10)
	case uint64:
		value = strconv.FormatUint(uint64(any.(uint64)), 10)
	case float32:
		value = strconv.FormatFloat(float64(any.(float32)), 'f', 2, 32)
	case float64:
		value = strconv.FormatFloat(any.(float64), 'f', 4, 64)
	case reflect.Value:
		if anyi.IsValid() && anyi.CanInterface() {
			return value2string(anyi.Interface())
		} else {
			return value2string(any)
		}
	default:
		success = false
	}
	return
}

func (s *Session) Command(name string, a ...interface{}) *Session {
	var args = make([]string, 0)
	// init cmd, args, dir, envs
	// if not init, program may panic
	s.inj.Map(name).Map(args).Map(s.dir).Map(map[string]string{})
	for _, v := range a {
		switch reflect.TypeOf(v).Kind() {
		case reflect.String:
			args = append(args, v.(string))
		case reflect.Array, reflect.Slice:
			{
				av := reflect.ValueOf(v)
				for i := 0; i < av.Len(); i++ {
					sval, succ := base2string(av.Index(i))
					if succ {
						args = append(args, sval)
					}
				}
			}
		default:
			sval, succ := base2string(v)
			if succ {
				args = append(args, sval)
			} else {
				s.inj.Map(v)
			}
		}
	}
	if len(args) != 0 {
		s.inj.Map(args)
	}
	s.inj.Invoke(s.appendCmd)
	return s
}

// combine Command and Run
func (s *Session) Call(name string, a ...interface{}) error {
	return s.Command(name, a...).Run()
}

/*
func (s *Session) Exec(cmd string, args ...string) error {
	return s.Call(cmd, args)
}
*/

func (s *Session) SetEnv(key, value string) *Session {
	s.Env[key] = value
	return s
}

func (s *Session) SetDir(dir string) *Session {
	s.dir = Dir(dir)
	return s
}

func (s *Session) SetInput(in string) *Session {
	s.Stdin = strings.NewReader(in)
	return s
}

func (s *Session) SetStdin(r io.Reader) *Session {
	s.Stdin = r
	return s
}

func (s *Session) SetTimeout(d time.Duration) *Session {
	s.timeout = d
	return s
}

func newEnviron(env map[string]string, inherit bool) []string { //map[string]string {
	environ := make([]string, 0, len(env))
	if inherit {
		for _, line := range os.Environ() {
			for k, _ := range env {
				if strings.HasPrefix(line, k+"=") {
					goto CONTINUE
				}
			}
			environ = append(environ, line)
		CONTINUE:
		}
	}
	for k, v := range env {
		environ = append(environ, k+"="+v)
	}
	return environ
}

func (s *Session) appendCmd(cmd string, args []string, cwd Dir, env map[string]string) {
	if s.started {
		s.started = false
		s.cmds = make([]*exec.Cmd, 0)
	}
	for k, v := range s.Env {
		if _, ok := env[k]; !ok {
			env[k] = v
		}
	}
	environ := newEnviron(s.Env, true) // true: inherit sys-env
	v, ok := s.alias[cmd]
	if ok {
		cmd = v[0]
		args = append(v[1:], args...)
	}
	c := exec.Command(cmd, args...)
	c.Env = environ
	c.Dir = string(cwd)
	s.cmds = append(s.cmds, c)
}
