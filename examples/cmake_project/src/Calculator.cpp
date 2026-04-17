#include "Calculator.hpp"
#include <iostream>
#include <cstring>

// C++ Calculator implementation - compiled with g++

Calculator::Calculator() : last_result(0), operation_count(0) {
}

Calculator::~Calculator() {
}

int Calculator::Add(int a, int b) {
    last_result = add(a, b);
    operation_count++;
    return last_result;
}

int Calculator::Subtract(int a, int b) {
    last_result = subtract(a, b);
    operation_count++;
    return last_result;
}

int Calculator::Multiply(int a, int b) {
    last_result = multiply(a, b);
    operation_count++;
    return last_result;
}

int Calculator::Divide(int a, int b) {
    last_result = divide(a, b);
    operation_count++;
    return last_result;
}

int Calculator::Power(int base, int exponent) {
    last_result = power(base, exponent);
    operation_count++;
    return last_result;
}

int Calculator::GetLastResult() const {
    return last_result;
}

int Calculator::GetOperationCount() const {
    return operation_count;
}

void Calculator::Reset() {
    last_result = 0;
    operation_count = 0;
}

void Calculator::PrintStats() const {
    std::cout << "Last result: " << last_result << std::endl;
    std::cout << "Total operations: " << operation_count << std::endl;
}
