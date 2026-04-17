#include <iostream>
#include "Calculator.hpp"

// C++ main program - compiled with g++

int main() {
    std::cout << "=== Buildozer C++ Calculator Example ===" << std::endl;
    std::cout << "Using C++ Calculator class wrapping C math library" << std::endl;
    std::cout << "Both compiled with Buildozer drivers (C with gcc, C++ with g++)" << std::endl << std::endl;

    // Create calculator instance
    Calculator calc;

    // Test basic arithmetic operations
    int a = 10;
    int b = 5;

    std::cout << "Testing basic arithmetic operations:" << std::endl;
    std::cout << "a = " << a << ", b = " << b << std::endl << std::endl;

    std::cout << "Add(" << a << ", " << b << ") = " << calc.Add(a, b) << std::endl;
    std::cout << "Subtract(" << a << ", " << b << ") = " << calc.Subtract(a, b) << std::endl;
    std::cout << "Multiply(" << a << ", " << b << ") = " << calc.Multiply(a, b) << std::endl;
    std::cout << "Divide(" << a << ", " << b << ") = " << calc.Divide(a, b) << std::endl;
    std::cout << "Power(" << a << ", " << b << ") = " << calc.Power(a, b) << std::endl;

    std::cout << std::endl;

    // Print statistics
    std::cout << "Calculator Statistics:" << std::endl;
    calc.PrintStats();

    std::cout << std::endl;

    // Test more complex sequence
    std::cout << "Testing sequence of operations:" << std::endl;
    calc.Reset();
    
    int result = calc.Add(100, 50);
    std::cout << "After Add(100, 50): " << result << std::endl;
    
    result = calc.Multiply(result, 2);
    std::cout << "After Multiply(result, 2): " << result << std::endl;
    
    result = calc.Divide(result, 3);
    std::cout << "After Divide(result, 3): " << result << std::endl;

    std::cout << std::endl;
    std::cout << "Final Statistics:" << std::endl;
    calc.PrintStats();

    std::cout << std::endl << "=== C++ Calculator Test Complete ===" << std::endl;

    return 0;
}
