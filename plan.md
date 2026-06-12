1. **Refactor memory leak vulnerabilities using `time.After` in loops (`retry/retry.go`):**
   - Replace the `time.After(delay)` in the `select` blocks of the `retry.Do` and `retry.DoWithValue` functions with explicitly instantiated and stopped `time.Timer`s using `time.NewTimer(delay)`. Inside the loop, instantiate the timer with `timer := time.NewTimer(delay)`, and call `timer.Stop()` when the select exits. This ensures timers don't leak memory and CPU cycles when contexts cancel early.
2. **Complete pre-commit steps to ensure proper testing, verification, review, and reflection are done.**
3. **Submit the code.**
