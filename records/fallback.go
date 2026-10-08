package records

import (
	"errors"
	"fmt"
	"io"
	"sync"
)

type Falling struct {
	To     Store
	Local  Local
	Warn   io.Writer
	warned sync.Map
}

func (f *Falling) Name() string { return f.To.Name() }

func (f *Falling) fell(kind Kind, err error) bool {
	if err == nil || errors.Is(err, ErrAbsent) {
		return false
	}
	if _, already := f.warned.LoadOrStore(kind, true); !already && f.Warn != nil {
		fmt.Fprintf(f.Warn, "mia: %s is not answering for %s — using .git/mia/ for now\n",
			f.To.Name(), kind)
	}
	return true
}

func (f *Falling) Get(kind Kind, key string) ([]byte, error) {
	body, err := f.To.Get(kind, key)
	if !f.fell(kind, err) {
		return body, err
	}
	return f.Local.Get(kind, key)
}

func (f *Falling) Put(kind Kind, key string, body []byte) error {
	err := f.To.Put(kind, key, body)
	if !f.fell(kind, err) {
		return err
	}
	return f.Local.Put(kind, key, body)
}

func (f *Falling) Append(kind Kind, key string, body []byte) error {
	err := f.To.Append(kind, key, body)
	if !f.fell(kind, err) {
		return err
	}
	return f.Local.Append(kind, key, body)
}

func (f *Falling) List(kind Kind, under string) ([]string, error) {
	keys, err := f.To.List(kind, under)
	if !f.fell(kind, err) {
		return keys, err
	}
	return f.Local.List(kind, under)
}

func (f *Falling) Open(kind Kind, key string) error {
	err := f.To.Open(kind, key)
	if !f.fell(kind, err) {
		return err
	}
	return f.Local.Open(kind, key)
}

func (f *Falling) URL(kind Kind, key string) (string, error) {
	url, err := f.To.URL(kind, key)
	if !f.fell(kind, err) {
		return url, err
	}
	return f.Local.URL(kind, key)
}

type Divergence struct {
	Kind   Kind   `json:"kind"`
	Key    string `json:"key"`
	Local  []byte `json:"-"`
	Remote []byte `json:"-"`
	Only   string `json:"only,omitempty"`
}
