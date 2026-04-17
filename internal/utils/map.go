package utils

import (
	"errors"
	"fmt"
	"sync"
)

// MapFunc is a function that transforms a value of type T to type U, potentially returning an error.
type MapFunc[T, U any] func(T) (U, error)

// Map applies a transformation function to each element in the input slice concurrently
// using a fork-join pattern. It returns the transformed slice or an error combining all
// encountered errors.
//
// Example:
//
//	numbers := []int{1, 2, 3, 4, 5}
//	results, errs := Map(numbers, func(n int) (string, error) {
//		return fmt.Sprintf("num_%d", n), nil
//	})
func Map[T, U any](input []T, fn MapFunc[T, U]) ([]U, error) {
	if len(input) == 0 {
		return []U{}, nil
	}

	results := make([]U, len(input))
	errs := make([]error, len(input))
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Fork: spawn a goroutine for each input element
	wg.Add(len(input))
	for i, item := range input {
		go func(idx int, val T) {
			defer wg.Done()
			result, err := fn(val)
			if err != nil {
				mu.Lock()
				errs[idx] = err
				mu.Unlock()
			} else {
				results[idx] = result
			}
		}(i, item)
	}

	// Join: wait for all goroutines to complete
	wg.Wait()

	// Aggregate all errors
	aggregatedErr := AggregateErrors(errs)
	if aggregatedErr != nil {
		return nil, aggregatedErr
	}

	return results, nil
}

// FilterMap applies a predicate and transformation function to each element concurrently.
// Elements that return an error or where the predicate returns false are filtered out.
//
// Example:
//
//	numbers := []int{1, 2, 3, 4, 5}
//	results, errs := FilterMap(numbers, func(n int) (string, bool, error) {
//		if n%2 == 0 { // Keep only even numbers
//			return fmt.Sprintf("even_%d", n), true, nil
//		}
//		return "", false, nil
//	})
func FilterMap[T, U any](
	input []T,
	fn func(T) (U, bool, error),
) ([]U, error) {
	if len(input) == 0 {
		return []U{}, nil
	}

	type resultItem struct {
		value   U
		include bool
		index   int
	}

	resultsChan := make(chan resultItem, len(input))
	errsChan := make(chan error, len(input))
	var wg sync.WaitGroup

	// Fork: spawn a goroutine for each input element
	wg.Add(len(input))
	for i, item := range input {
		go func(idx int, val T) {
			defer wg.Done()
			result, include, err := fn(val)
			if err != nil {
				errsChan <- err
			} else if include {
				resultsChan <- resultItem{value: result, include: true, index: idx}
			}
		}(i, item)
	}

	// Join: wait for all goroutines to complete
	wg.Wait()
	close(resultsChan)
	close(errsChan)

	// Collect errors
	var allErrs []error
	for err := range errsChan {
		allErrs = append(allErrs, err)
	}

	if len(allErrs) > 0 {
		return nil, AggregateErrors(allErrs)
	}

	// Collect and preserve order of results
	results := make([]U, 0, len(input))
	resultsMap := make(map[int]U)
	for item := range resultsChan {
		resultsMap[item.index] = item.value
	}

	// Reconstruct in original order (for included items)
	for i := 0; i < len(input); i++ {
		if v, ok := resultsMap[i]; ok {
			results = append(results, v)
		}
	}

	return results, nil
}

// ForEach applies a function to each element concurrently and waits for all to complete.
// It returns a combined error if any operations fail.
//
// Example:
//
//	items := []string{"a", "b", "c"}
//	err := ForEach(items, func(item string) error {
//		return processItem(item)
//	})
func ForEach[T any](input []T, fn func(T) error) error {
	if len(input) == 0 {
		return nil
	}

	errsChan := make(chan error, len(input))
	var wg sync.WaitGroup

	// Fork: spawn a goroutine for each input element
	wg.Add(len(input))
	for _, item := range input {
		go func(val T) {
			defer wg.Done()
			if err := fn(val); err != nil {
				errsChan <- err
			}
		}(item)
	}

	// Join: wait for all goroutines to complete
	wg.Wait()
	close(errsChan)

	// Collect and aggregate errors
	var errs []error
	for err := range errsChan {
		errs = append(errs, err)
	}

	return AggregateErrors(errs)
}

// AggregateErrors combines multiple errors into a single error.
// Returns nil if the slice is empty or contains only nil values.
// Returns the single error if only one non-nil error exists.
// Returns a MultiError wrapping all errors if multiple errors exist.
func AggregateErrors(errs []error) error {
	var nonNilErrs []error
	for _, err := range errs {
		if err != nil {
			nonNilErrs = append(nonNilErrs, err)
		}
	}

	if len(nonNilErrs) == 0 {
		return nil
	}

	if len(nonNilErrs) == 1 {
		return nonNilErrs[0]
	}

	return &MultiError{Errors: nonNilErrs}
}

// MultiError represents multiple errors that occurred during a concurrent operation.
type MultiError struct {
	Errors []error
}

// Error implements the error interface.
func (me *MultiError) Error() string {
	if len(me.Errors) == 0 {
		return ""
	}
	if len(me.Errors) == 1 {
		return me.Errors[0].Error()
	}
	return fmt.Sprintf("%d errors occurred: %v", len(me.Errors), me.Errors)
}

// Is implements the errors.Is interface for checking if any of the wrapped errors match.
func (me *MultiError) Is(target error) bool {
	for _, err := range me.Errors {
		if errors.Is(err, target) {
			return true
		}
	}
	return false
}

// As implements the errors.As interface for extracting the first error of a specific type.
func (me *MultiError) As(target interface{}) bool {
	for _, err := range me.Errors {
		if errors.As(err, target) {
			return true
		}
	}
	return false
}
