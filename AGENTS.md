# AGENTS.md

These rules apply to any language and any framework. Where a repository has its own `AGENTS.md`, its rules win. Where it has established conventions, follow them. Most rules give their reason, so you can apply them with judgment when a case does not fit.

The first three sections set the frame. The rest follow the order of the work: design, code, testing, enforcement, and the way you work.

## Write idiomatic code

Write the code that a fluent programmer of the language and the framework would write. These rules say what must be true, and the idiom decides how the code says it. For example, the rule says to handle every error. Rust handles it with a `Result`, Go checks `err`, and Elixir lets a supervised process crash. When the toolchain enforces an idiom, follow it. The toolchain includes the formatter, the compiler, the ecosystem's default linter, and any structure the framework requires.

## Rank goals: safety, then performance, then developer experience

Safety means the program does the right thing or stops. Performance comes second because a slow design cannot be fixed later without a rewrite. Developer experience comes third, and it matters only because it serves the first two. Simplicity is how you get all three at once. It is not a trade against them.

## Choose simple over easy

- Simple means not interleaved: one role, one concept, one dimension. Easy means familiar or close at hand. Choose simple when the two disagree. Easy and tangled code is cheap to write and expensive to keep.
- Separate essential complexity from accidental complexity. Essential complexity is what the user's problem requires. Everything else, such as state the problem does not need and layers that only forward calls, is accidental. Remove it.
- Complexity shows up in three ways: one change needs edits in many places, a reader has to hold too much in their head, or a reader cannot tell what they need to know. Treat each one as a defect.
- Design it twice. The first idea is rarely the simplest. Before you add a module, an interface, or a data layout, sketch two designs that differ in structure, not only in names. Compare them and pick the simpler one.
- Expect simplicity to come from the last revision, not the first draft. Be ready to throw a draft away.

## Design the data first

- Before you write logic, write down the data. Note its shape, how many items there are, how they are read and written, how long they live, and which code owns them. When you know the data, the code that transforms it is usually obvious.
- Design for many, not one. Where there is one item, there are usually many, so write functions that take a batch.
- Design these up front because they are expensive to change: data layout, lifetimes, limits, and module boundaries. Leave everything else to emerge from working code.
- Group data by lifetime. A request, a frame, a job, or a session owns its data, and all of that data is released together. Tracking a lifetime per object produces leaks, use-after-free bugs, and cleanup code that drifts out of sync. In garbage-collected languages, the same rule decides which object holds the references.
- Put a limit on everything that grows: queues, buffers, retries, loops, input sizes, and recursion depth. Name each limit and say what happens when a value reaches it. An unbounded queue or buffer eventually exhausts memory under load.

## Design for performance, then measure

- Make performance decisions during design, where the large wins are. Afterward, only small ones are left.
- Estimate costs on paper before you choose a design. Check network, then disk, then memory, then CPU, for both bandwidth and latency. Being roughly right is enough to rule out designs that are wrong by 1000x.
- Know what the hardware does with your data. Prefer contiguous arrays to structures that chase pointers, because memory access usually costs more than computation.
- Separate the control plane from the data plane. Batch work to amortize the cost of each network call, disk access, and lock.
- Measure before you claim a speedup. Name what limits the number, and confirm that the benchmark ran the work you think it ran.

## Draw module boundaries around decisions, not steps

- Give each module one design decision that is likely to change, such as a file format, an algorithm, or a storage choice, and hide it. Do not split modules by the steps of a flowchart. A change to one decision then edits one module.
- Make modules deep, with a small interface over a lot of behavior. A module whose interface is as complex as its implementation costs more than it saves. Deep does not mean one entry point. If a high-level function is built from lower-level functions that callers can use on their own, keep those public, so a caller who needs more control can step down one level instead of rewriting.
- Design an interface for the requirements you know, including the ones that come later, and implement only what is needed now. Callers keep working when the implementation improves. Do not add parameters for requirements you are guessing at.
- Never hide cost or lifetime. If a call allocates, blocks, does I/O, takes a lock, or keeps a reference to its argument, its name or its signature says so. Hidden costs come back as slow code and as code that does not compose.
- Draw boundaries around systems that transform data, not around nouns in the domain. Do not build inheritance hierarchies that model the real world, such as `Animal` and `Dog`. Group code by the work it does on a set of data.
- Pull complexity down into the module. One author handling a hard case once is cheaper than every caller handling it.
- Define errors out of existence where you can. Shape the API so that an error case cannot happen. For example, deleting a missing key succeeds, and a substring past the end is empty.
- Pass options explicitly at the call site instead of relying on a library's defaults. Defaults change between versions, and the reader cannot see them.

## Let abstractions come from working code

- Write the concrete case first. Extract shared code when the same pattern appears a second time, from the real uses. Code that is made reusable before it is used is shaped wrong for the second use.
- Give every extracted function a real name and a real interface. Do not split a function only to make it shorter. A long function that reads top to bottom is better than ten fragments that each have one caller.
- Remove before you add. Delete dead code, unused options, and redundant checks first, then build on the smaller base.

## Keep logic pure and state in few owners

- Put decisions in pure functions that take plain data and return plain data. Keep mutable state in a small number of owners. Owners apply changes in place, in bulk, with explicit lifetimes. Pure functions are trivial to test, and a few owners make state easy to find.
- Centralize control flow in the parent function. Move conditions up to the caller and loops down into the callee. The parent holds the state, helpers compute changes, and leaf functions are pure.
- Keep one source of truth for each fact. Derive everything else from it. Do not copy or alias state, because copies go stale.
- Declare each variable in the smallest scope that works. Check a value close to where you use it.
- Let the framework own its parts, such as routes, models, and components. Call the pure functions from that framework code.
- Use simple, explicit control flow. Use recursion only when the depth has a proven bound. Tail recursion that the language guarantees to optimize counts as a loop, and a loop needs a limit.

## Assert invariants and handle every error

- Separate programmer errors from operating errors. A programmer error is a broken invariant. Assert it and crash, because continuing on corrupt state spreads the damage. An operating error is a network failure, a full disk, or bad input. Handle it, and test the code that handles it.
- Assert preconditions, postconditions, and invariants. Assert what must not happen as well as what must. Write `assert(a); assert(b);`, not `assert(a and b)`, so a failure names the exact condition.
- Validate external input once, at the boundary: the CLI, configuration, the network, and external APIs. Inside the boundary, trust the types.
- Handle every error that the API design cannot remove. Do not ignore a return value or catch an exception and continue as if nothing happened.

## Name things precisely

- Spend time on nouns and verbs. The name is the only description of the code that a reader gets.
- Use the language's case style and its conventional short names, such as `ctx` and `err` in Go. Do not abbreviate anything else. Put units and qualifiers last, most significant first: `latency_ms_max`, not `max_latency`.

## Write no comments

Code comments are banned, and that includes doc comments. The ban holds even in a repository whose convention is to write comments. A comment is prose that no tool checks, so it drifts from the code and nothing fails when it does.

- Put each piece of information where a tool checks it. What the code does goes in names. An assumption goes in an assertion. Expected behavior goes in a test. A constraint goes in a type. The reason for a change goes in a short commit message.
- When you want to write a comment, change the code until you no longer need one. Rename the variable, extract a function with a real name, or add an assertion.
- In code you change, delete the existing comments. Move any fact they hold into a name, an assertion, or a test first.
- Text that a tool reads is not a comment. Shebangs, compiler and linter directives, build tags, and license headers that a license requires are allowed. So are doc comments that a tool runs or checks, such as Rust doctests, and doc comments that the repository's linter requires.

## Treat each dependency as a cost

A dependency adds code you did not review, upgrades you did not plan, and an attack path you do not control. Add one only when it saves more than that. Prefer the standard library and a small set of well-known tools.

Wrap a dependency when it is a decision that is likely to change, such as a platform, a storage engine, a vendor API, or a networking library. The wrapper exposes only what the program uses, in the program's own terms. Do not wrap the standard library or a framework that owns the program's structure.

## Test behavior and make every failure replayable

- Inject time, randomness, and I/O so that any failure can be replayed from a seed.
- Test what callers can observe, not internal structure. A test that breaks when only the internals change gives no information.
- Do not write change-detector tests. A change-detector test repeats the implementation, for example by mocking every collaborator and checking that the code calls them in order. It fails on every refactor and passes when the behavior is wrong. Use real collaborators or simple fakes, and assert on the result. Delete change-detector tests you find in code you change.
- Test invalid input, and test data that changes from valid to invalid.
- A passing fuzzer shows that it found no bugs, not that there are none.

## Enforce rules with structure, not text

Each rule is only as strong as what enforces it. In descending priority:

1. **The codebase.** Make the mistake impossible. Give each piece of state one owner, give each task one supported way, and use types that cannot hold an illegal state.
2. **Static analysis.** If bad code still compiles, add a lint or CI check. Its error message names the function, type, or file to use instead.
3. **Prose.** Write a rule in documentation or in this file only when it needs judgment. Nothing fails when an agent skips it.
4. **Human review.** A reviewer catches what the other three miss, but a reviewer also misses things. This level is the least reliable.

When the user corrects the same mistake twice, propose a check one level up. Build it when the user agrees, and prove that it fails on the real past mistake. Delete the prose rule when a check replaces it.

## Work in small, verified steps

1. Read before you write: the code you will change, its callers, and its tests.
2. Restate the goal if the request is unclear. Ask the user only about decisions that are theirs to make. Look up facts yourself.
3. Make the smallest change that leaves the design as if the requirement had been there from the start. If the design no longer fits, fix the design first.
4. Fix the root cause. Trace the symptom back to the first place the program went wrong. A guard is a fix only when the state it handles is legitimate.
5. When you replace an internal API, migrate every caller and delete the old API in the same change. Do not leave a compatibility layer.
6. Verify against the real program. Run it and read the output. "It compiles" and "the tests pass" are not proof that the feature works.
7. Ask before you do anything that is hard to reverse or visible to others: deleting data, rewriting history, publishing, or sending messages.
8. Report what you ran, what failed, and what you skipped.
