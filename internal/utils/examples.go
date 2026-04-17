package utils

/*
Examples of using the fork-join pattern utilities.

Map Example - Transform a slice with error handling:

	numbers := []int{1, 2, 3, 4, 5}
	results, err := Map(numbers, func(n int) (string, error) {
		return fmt.Sprintf("num_%d", n), nil
	})
	if err != nil {
		log.Fatal(err) // Handle aggregated errors
	}
	// results: ["num_1", "num_2", "num_3", "num_4", "num_5"]


FilterMap Example - Transform and filter in parallel:

	items := []string{"apple", "banana", "cherry", "date"}
	results, err := FilterMap(items, func(s string) (string, bool, error) {
		if len(s) > 5 { // Keep only long words
			return strings.ToUpper(s), true, nil
		}
		return "", false, nil
	})
	if err != nil {
		log.Fatal(err)
	}
	// results: ["BANANA", "CHERRY"]


ForEach Example - Execute the same operation on all items:

	files := []string{"file1.txt", "file2.txt", "file3.txt"}
	err := ForEach(files, func(filename string) error {
		data, err := os.ReadFile(filename)
		if err != nil {
			return err
		}
		return processData(data)
	})
	if err != nil {
		// err might be a MultiError with all accumulated errors
		log.Fatal(err)
	}


Error Handling:

The utils package automatically aggregates errors from all concurrent operations:

- If no errors occur: returns nil
- If exactly one error occurs: returns that error directly
- If multiple errors occur: returns a *MultiError with all errors

Use errors.Is() to check for specific errors:

	if errors.Is(err, io.EOF) {
		// handle EOF
	}

Use errors.As() to extract errors of specific types.
*/
