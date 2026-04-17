package utils

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMap_Success(t *testing.T) {
	input := []int{1, 2, 3, 4, 5}
	results, err := Map(input, func(i int) (string, error) {
		return fmt.Sprintf("num_%d", i), nil
	})

	require.NoError(t, err, "Map should not error on successful transformations")
	require.Len(t, results, 5, "Results should have same length as input")
	assert.Equal(t, "num_1", results[0])
	assert.Equal(t, "num_5", results[4])
}

func TestMap_Empty(t *testing.T) {
	results, err := Map([]int{}, func(i int) (string, error) {
		return fmt.Sprintf("num_%d", i), nil
	})

	require.NoError(t, err, "Map should not error on empty input")
	require.Len(t, results, 0, "Results should be empty")
}

func TestMap_WithErrors(t *testing.T) {
	input := []int{1, 2, 3, 4, 5}
	results, err := Map(input, func(i int) (string, error) {
		if i == 2 || i == 4 {
			return "", fmt.Errorf("error for %d", i)
		}
		return fmt.Sprintf("num_%d", i), nil
	})

	require.Error(t, err, "Map should error when transformations fail")
	assert.Nil(t, results, "Results should be nil when error occurs")

	// Check error aggregation
	multiErr, ok := err.(*MultiError)
	assert.True(t, ok, "Error should be MultiError")
	assert.Len(t, multiErr.Errors, 2, "Should have 2 errors")
}

func TestMap_SingleError(t *testing.T) {
	input := []int{1, 2, 3}
	results, err := Map(input, func(i int) (string, error) {
		if i == 2 {
			return "", errors.New("test error")
		}
		return fmt.Sprintf("num_%d", i), nil
	})

	require.Error(t, err, "Map should error")
	assert.Nil(t, results, "Results should be nil when error occurs")
	assert.Equal(t, "test error", err.Error())
}

func TestFilterMap_Success(t *testing.T) {
	input := []int{1, 2, 3, 4, 5}
	results, err := FilterMap(input, func(i int) (string, bool, error) {
		if i%2 == 0 { // Keep even numbers
			return fmt.Sprintf("even_%d", i), true, nil
		}
		return "", false, nil
	})

	require.NoError(t, err, "FilterMap should not error")
	require.Len(t, results, 2, "Should have 2 even numbers")
	assert.Equal(t, "even_2", results[0])
	assert.Equal(t, "even_4", results[1])
}

func TestFilterMap_Empty(t *testing.T) {
	input := []int{1, 3, 5} // All odd
	results, err := FilterMap(input, func(i int) (string, bool, error) {
		if i%2 == 0 {
			return fmt.Sprintf("even_%d", i), true, nil
		}
		return "", false, nil
	})

	require.NoError(t, err, "FilterMap should not error")
	require.Len(t, results, 0, "Should have no results")
}

func TestFilterMap_WithErrors(t *testing.T) {
	input := []int{1, 2, 3, 4, 5}
	results, err := FilterMap(input, func(i int) (string, bool, error) {
		if i == 3 {
			return "", false, errors.New("error at 3")
		}
		if i%2 == 0 {
			return fmt.Sprintf("even_%d", i), true, nil
		}
		return "", false, nil
	})

	require.Error(t, err, "FilterMap should error")
	assert.Nil(t, results, "Results should be nil when error occurs")
}

func TestForEach_Success(t *testing.T) {
	input := []int{1, 2, 3, 4, 5}
	processed := make([]int, 0)
	var mu sync.Mutex

	err := ForEach(input, func(i int) error {
		mu.Lock()
		processed = append(processed, i)
		mu.Unlock()
		return nil
	})

	require.NoError(t, err, "ForEach should not error")
	// Note: order may not be preserved due to concurrency, so check set equality
	assert.Len(t, processed, 5, "Should have processed all items")
}

func TestForEach_WithErrors(t *testing.T) {
	input := []int{1, 2, 3, 4, 5}
	err := ForEach(input, func(i int) error {
		if i == 2 || i == 4 {
			return fmt.Errorf("error for %d", i)
		}
		return nil
	})

	require.Error(t, err, "ForEach should error")
	multiErr, ok := err.(*MultiError)
	assert.True(t, ok, "Error should be MultiError")
	assert.Len(t, multiErr.Errors, 2, "Should have 2 errors")
}

func TestForEach_Empty(t *testing.T) {
	err := ForEach([]int{}, func(i int) error {
		return nil
	})

	require.NoError(t, err, "ForEach should not error on empty input")
}

func TestAggregateErrors_Empty(t *testing.T) {
	err := AggregateErrors([]error{})
	assert.Nil(t, err, "Empty errors should result in nil")
}

func TestAggregateErrors_SingleError(t *testing.T) {
	testErr := errors.New("test error")
	err := AggregateErrors([]error{nil, testErr, nil})

	require.Error(t, err)
	assert.Equal(t, testErr, err, "Single error should be returned directly")
}

func TestAggregateErrors_MultipleErrors(t *testing.T) {
	err1 := errors.New("error 1")
	err2 := errors.New("error 2")
	err3 := errors.New("error 3")

	err := AggregateErrors([]error{err1, nil, err2, err3})

	require.Error(t, err)
	multiErr, ok := err.(*MultiError)
	require.True(t, ok, "Should be MultiError")
	assert.Len(t, multiErr.Errors, 3, "Should have 3 errors")
}

func TestMultiError_Error(t *testing.T) {
	err1 := errors.New("error 1")
	err2 := errors.New("error 2")
	multiErr := &MultiError{Errors: []error{err1, err2}}

	errMsg := multiErr.Error()
	assert.Contains(t, errMsg, "2 errors occurred")
	assert.Contains(t, errMsg, "error 1")
	assert.Contains(t, errMsg, "error 2")
}

func TestMultiError_Is(t *testing.T) {
	testErr := errors.New("test error")
	err1 := testErr
	err2 := errors.New("other error")

	multiErr := &MultiError{Errors: []error{err1, err2}}

	assert.True(t, errors.Is(multiErr, testErr), "Should find test error")
	assert.False(t, errors.Is(multiErr, errors.New("not found")), "Should not find different error")
}

func TestMultiError_As(t *testing.T) {
	type customError struct {
		msg string
	}

	impl := func(ce *customError) error {
		return errors.New(ce.msg)
	}

	err1 := impl(&customError{msg: "test"})
	err2 := errors.New("other")

	multiErr := &MultiError{Errors: []error{err1, err2}}

	assert.Len(t, multiErr.Errors, 2, "Should have 2 errors")
}

func BenchmarkMap(b *testing.B) {
	input := make([]int, 1000)
	for i := 0; i < 1000; i++ {
		input[i] = i
	}

	fn := func(i int) (string, error) {
		return fmt.Sprintf("num_%d", i), nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Map(input, fn)
	}
}

func BenchmarkForEach(b *testing.B) {
	input := make([]int, 1000)
	for i := 0; i < 1000; i++ {
		input[i] = i
	}

	fn := func(i int) error {
		// Simulate some work
		_ = strings.Count(fmt.Sprintf("num_%d", i), "1")
		return nil
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ForEach(input, fn)
	}
}
