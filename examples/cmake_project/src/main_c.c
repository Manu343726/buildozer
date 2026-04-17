#include <stdio.h>
#include "math.h"

// C main program - compiled with gcc

int main() {
    printf("=== Buildozer C Calculator Example ===\n");
    printf("Using C math library compiled with Buildozer gcc driver\n\n");

    // Test basic arithmetic operations
    int a = 10;
    int b = 5;

    printf("Testing basic arithmetic operations:\n");
    printf("a = %d, b = %d\n\n", a, b);

    printf("add(%d, %d) = %d\n", a, b, add(a, b));
    printf("subtract(%d, %d) = %d\n", a, b, subtract(a, b));
    printf("multiply(%d, %d) = %d\n", a, b, multiply(a, b));
    printf("divide(%d, %d) = %d\n", a, b, divide(a, b));
    printf("power(%d, %d) = %d\n", a, b, power(a, b));

    printf("\n");

    // Test edge cases
    printf("Testing edge cases:\n");
    printf("divide(10, 0) = %d (division by zero)\n", divide(10, 0));
    printf("power(2, 10) = %d\n", power(2, 10));
    printf("power(3, 0) = %d\n", power(3, 0));

    printf("\n=== C Calculator Test Complete ===\n");

    return 0;
}
