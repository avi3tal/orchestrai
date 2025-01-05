OnCondition (Multi-Branch)
==========================

•	Filename: on_condition_example.go
•	Key DSL Method:
•	OnCondition(conditionFunc, map[string]Agent[T])

Overview
--------

This example demonstrates multi-branch conditional routing. Instead of a simple binary ThenIf(...), the OnCondition(...) method can route to any number of branches based on a branch key returned by conditionFunc.

Scenario
--------

•	We have an AccountState with a PlanType field ("free", "paid", or "enterprise").
•	A “CheckUser” agent logs the current plan.
•	OnCondition(...) checks PlanType and routes to:
•	UpgradeFreeAgent (if PlanType == "free")
•	PaidWelcomeAgent (if PlanType == "paid")
•	EnterpriseFlow (if PlanType == "enterprise")
•	If the plan type is unknown, we do not define a matching branch in the map, so the workflow can end or choose a fallback route.

How to Run
----------

```shell
cd examples/oncondition
go run main.go
```

Expected Output
---------------

•	Runs multiple times with different initial states (PlanType = "free", "paid", "enterprise", or "weird").
•	Each plan flows to a different branch agent.
•	Logs show the selected branch and subsequent actions.

