#ifndef MATH_H
#define MATH_H

#ifdef __cplusplus
extern "C" {
#endif

// C math library - compiled with gcc
int add(int a, int b);
int subtract(int a, int b);
int multiply(int a, int b);
int divide(int a, int b);
int power(int base, int exponent);

#ifdef __cplusplus
}
#endif

#endif // MATH_H
