#ifndef CALCULATOR_HPP
#define CALCULATOR_HPP

#include "math.h"

// C++ Calculator class - compiled with g++
// Wraps C math functions with object-oriented interface
class Calculator {
private:
    int last_result;
    int operation_count;

public:
    Calculator();
    ~Calculator();

    int Add(int a, int b);
    int Subtract(int a, int b);
    int Multiply(int a, int b);
    int Divide(int a, int b);
    int Power(int base, int exponent);

    int GetLastResult() const;
    int GetOperationCount() const;
    void Reset();

    void PrintStats() const;
};

#endif // CALCULATOR_HPP
