# Simple Graph Example

This example demonstrates a basic graph execution flow using the OrchestAI DAG implementation.

## Flow Description

The graph processes a number through the following steps:
1. Double the value
2. Add 10
3. Divide by 3

## Running the Example

```bash
go run main.go
```

Initial value: 5
Final value: 6
Execution time: XXms


```angular2html
graph LR
    START --> double
    double --> add_ten
    add_ten --> divide_by_three
    divide_by_three --> END

    style START fill:#f9f,stroke:#333
    style END fill:#f9f,stroke:#333
```


Key Concepts Demonstrated

Basic graph creation and configuration
Node implementation
State management
Graph compilation and execution
Error handling


This example demonstrates:

1. Basic graph construction
2. Node implementation
3. State management
4. Graph execution
5. Error handling
6. Debug output
