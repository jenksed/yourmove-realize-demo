# Role-intelligence evaluation (2026-10-26, muse-spark-1.3-contributor)

Input: 2 synthetic reviewed records (MySQL diagnosis r2 unknown outcome; billing escalation r1 recorded) + weak-only wiki record + corrected-attribution variant. 4 requests, 0 failures at shipped 3072/low-reasoning profile.

- support/full: ev1 relevant (caps quote diagnosis actions; gaps: no Linux shown, DBA applied change, outcome unknown); ev2 irrelevant for TSE. Unmet: account ownership/renewals only.
- account/full: ev1 irrelevant (no customer explanation, no coordination); ev2 relevant with transferable caps (plain-language explanation, billing coordination) + explicit non-ownership gaps. Unmet: cross-team engineering coordination, prod DB changes.
- support/weak-only: correctly all unmet; wiki record irrelevant.
- support/corrected-attribution: caps narrowed to what the correction supports; fix ownership moved to gaps; outcome still unknown.

Finding: same history yields substantively different role readings. 2048-token cap failed 2/4 turns (finish=length); 3072 (the shipped discovery profile) completed 4/4. Prompt strategy kept: bounded JSON, verbatim unmet echo, irrelevant records carry no capabilities.
