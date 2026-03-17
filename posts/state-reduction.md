---
title: Do we really need the extra state?
date: 2026-03-17
description: Structural exploration of the idea of State Reduction.
cover: state-reduction.png
---

When designing hardware, every component of state added to the system has a visible cost in space, power, and complexity. This is why the idea of state reduction, when it comes to hardware engineering, comes very naturally, as the trade-offs are immediately visible. In contrast, software is invisible and unvisualizable (Brooks, _No Silver Bullet_, 1986). This essential difficulty hides the cost of growing state space of our programs. The idea of state reduction is nothing groundbreaking, and comes naturally with experience to software developers, and at this point is a very fundamental aspect of the field, but I still find myself having to explain it frequently. This is why I wanted to put it into a structured form and explore it deeply within a blog post as a future reference.

A program or a subset of it can exist in multiple state variants. The number of these variants will increase combinatorially as we keep on adding new features and consequently increasing the state space. The important part to grasp in this is that this has an inverse relationship with the maintainability and reliability of the system. When you think about it, bugs in a program are states that are not handled by the specification. No matter how much we try to make "impossible states impossible," either through logic or other means, after a certain point we realize that an invalid variant will eventually hit, and we will find ourselves looking confused at the debugger. Even if we are smart enough to think of that one edge case, all our extra logic to cover the invariant is usually accidental complexity, which also damages the quality of the code regardless of how well we are covered in terms of bugs.

The above graph illustrates the dangerous aspect of the combinatoric nature of the state. We could feel safe looking at the number of extra features increasing linearly, and we might feel like we have it under control, but all the while the number of state variants increases exponentially. This means the bug surface area, the variants to test, and the cognitive load on the developer also increase exponentially. How reliably can we keep this under control? Is this worth the comfort of that extra combination?

The idea is simple. We should be practicing state reduction heavily and asking ourselves at all times: do we really need that extra state?
