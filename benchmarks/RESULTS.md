# Synthetic benchmark results (2026-10-26, muse-spark-1.3-contributor)

Input: benchmarks/synthetic-resume.txt (5 ordinary bullets). Limit: 8 requests/run.

- Run 1: 5 leads; adaptive Q2 (DBA handoff); call 4 finish=length at 2048, no proposal.
- Run 2 (1 retry): 4 leads; Q2 verified resolution, Q3 handoff contents; source-valid proposal; attribution preserved (DBA applied change); unknown timing stayed unresolved.
- In-app Standard (muse-spark-1.3): 5 leads; proposal after budget raised to 3072; earlier 2048 attempts failed finish=length twice, retry recovered, no input lost.
- Cost signal: ~650-760 input / ~2000 output tokens per proposal turn (~$0.009 estimated Standard).
