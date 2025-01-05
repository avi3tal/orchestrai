Parallel + Join
================

•	Filename: parallel_join_example.go
•	Key DSL Methods:
•	ThenAll(agentA, agentB)
•	.Join(func(ctx context.Context, states []T, cfg types.Config[T]) (types.NodeResponse[T], error))

Overview
--------

This example shows how to run multiple agents in parallel and then join their results. After a ThenAll(...) call, the parallel branches execute concurrently. Once all complete, the DSL calls .Join(...) to aggregate their outputs into a single state, which is then passed to the next step in the flow.

Scenario
--------

•	Agent GetEmail retrieves a user’s email address.
•	Agent GetAddress fetches the user’s shipping address.
•	Both run in parallel.
•	We join them, merging the partial states (email + address).
•	Finally, a DoSomething agent sees the combined data and proceeds.


How to Run
----------
```shell
cd examples/parallel_join
go run parallel_join_example.go
```

Expected Output
---------------


•	Console prints indicating the steps:
1.	GetEmail sets Email in the state.
2.	GetAddress sets Address.
3.	The Join function merges them.
4.	DoSomething sees the combined state.