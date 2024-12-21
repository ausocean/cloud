package registry

import (
	"errors"
	"sync"
	"testing"
)

func cleanup() {
	instantiated = nil
	once = sync.Once{}
}

func TestRegistrySingleton(t *testing.T) {
	registry1 := get()
	registry2 := get()
	if registry1 != registry2 {
		t.Errorf("expected registry1 to be equal to registry2")
	}
	t.Cleanup(cleanup)
}

var ErrInvalidArgumentType = errors.New("invalid argument type")

type testType struct {
	str string
}

func (testType) Name() string {
	return "testType"
}

func newTestType(str string) *testType {
	return &testType{str}
}

func (o testType) New(args ...interface{}) (any, error) {
	var str string
	for _, arg := range args {
		if _, ok := arg.(string); !ok {
			return nil, ErrInvalidArgumentType
		}
		str = arg.(string)
	}
	return newTestType(str), nil
}

func TestRegistryRegisterAndGet(t *testing.T) {
	err := Register(testType{})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	obj, err := Get("testType", "hello")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	testObj := obj.(*testType)
	if testObj.str != "hello" {
		t.Errorf("expected testType.str to be hello, got %s", testObj.str)
	}
	t.Cleanup(cleanup)
}

type testTypeNoNew struct {
	str string
}

func (testTypeNoNew) Name() string {
	return "testTypeNoNew"
}

func newTestObjectNoNew(str string) *testTypeNoNew {
	return &testTypeNoNew{str}
}

func TestRegistryRegisterAndGetNoNewPointer(t *testing.T) {
	obj := &testTypeNoNew{str: "pointer"}
	err := Register(obj)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	valObj, err := Get("testTypeNoNew")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if valObj.(*testTypeNoNew) != obj {
		t.Errorf("expected same pointer, got different instances")
	}
	t.Cleanup(cleanup)
}

func TestRegistryRegisterAndGetNoNewValue(t *testing.T) {
	obj := testTypeNoNew{str: "pointer"}
	err := Register(obj)
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	valObj, err := Get("testTypeNoNew")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if valObj.(testTypeNoNew) != obj {
		t.Errorf("expected same pointer, got different instances")
	}
	t.Cleanup(cleanup)
}

func TestRegistryDuplicateRegister(t *testing.T) {
	err := Register(testType{})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	err = Register(testType{})
	if err == nil || !errors.As(err, &ErrTypeAlreadyRegistered{}) {
		t.Errorf("expected ErrTypeAlreadyRegistered, got %v", err)
	}
	t.Cleanup(cleanup)
}

func TestRegistryGetUnregistered(t *testing.T) {
	_, err := Get("unregisteredObject")
	if err == nil || !errors.As(err, &ErrTypeNotRegistered{}) {
		t.Errorf("expected ErrTypeNotRegistered, got %v", err)
	}
	t.Cleanup(cleanup)
}

type unnamedObject struct{}

func TestRegistryRegisterNonNameable(t *testing.T) {
	err := Register(unnamedObject{})
	if err == nil {
		t.Error("expected an error for non-Named type")
	}
	t.Cleanup(cleanup)
}

func TestRegistryThreadSafety(t *testing.T) {
	iterations := 1000
	wg := sync.WaitGroup{}
	wg.Add(iterations * 2)

	for i := 0; i < iterations; i++ {
		go func() {
			defer wg.Done()
			_ = Register(testType{})
		}()
		go func() {
			defer wg.Done()
			_, _ = Get("testType", "hello")
		}()
	}

	wg.Wait()
	t.Cleanup(cleanup)
}

type errorNewableObject struct{}

func (errorNewableObject) Name() string {
	return "errorNewableObject"
}

func (errorNewableObject) New(args ...interface{}) (any, error) {
	return nil, errors.New("new failed")
}

func TestRegistryNewableError(t *testing.T) {
	err := Register(errorNewableObject{})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	_, err = Get("errorNewableObject")
	if err == nil || err.Error() != "error call New for type errorNewableObject: new failed" {
		t.Errorf("expected New error, got %v", err)
	}
	t.Cleanup(cleanup)
}

type stringerObject struct{}

func (stringerObject) String() string {
	return "stringerObject"
}

func TestRegistryStringerFallback(t *testing.T) {
	err := Register(stringerObject{})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	_, err = Get("stringerObject")
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	t.Cleanup(cleanup)
}

func TestRegistrySingletonThreadSafety(t *testing.T) {
	wg := sync.WaitGroup{}
	wg.Add(10)

	var instances []*registry
	mu := sync.Mutex{}

	for i := 0; i < 10; i++ {
		go func() {
			defer wg.Done()
			r := get()
			mu.Lock()
			instances = append(instances, r)
			mu.Unlock()
		}()
	}

	wg.Wait()

	for i := 1; i < len(instances); i++ {
		if instances[i] != instances[0] {
			t.Errorf("expected singleton instance, got different instances")
		}
	}
	t.Cleanup(cleanup)
}

func TestRegistryRegisterNil(t *testing.T) {
	err := Register(nil)
	if err == nil {
		t.Errorf("expected error when registering nil")
	}
	t.Cleanup(cleanup)
}

func TestRegistryInvalidArgumentType(t *testing.T) {
	err := Register(testType{})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	_, err = Get("testType", 123) // Passing a non-string argument
	if err == nil || !errors.Is(err, ErrInvalidArgumentType) {
		t.Errorf("expected ErrInvalidArgumentType, got %v", err)
	}
	t.Cleanup(cleanup)
}
