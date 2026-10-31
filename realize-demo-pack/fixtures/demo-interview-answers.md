# Synthetic narrative notes — NOT model output

**Candidate:** Avery Morgan (invented). The provided résumé is synthetic. These are true facts *within the fictional scenario*, intended to help an operator answer whatever real Muse questions are asked.

### Incident: slow MySQL application

The candidate helped investigate an application that had become slow in a Linux-based hosting environment. They looked at available slow-query logs and ran `EXPLAIN` against an affected query. The plan indicated a full table scan. They documented the evidence, explained why it was relevant, and passed the query and findings to a database engineering team. They did **not** choose or implement the final fix. They did not see verified performance metrics afterward and should not claim an outcome.

**Possible natural answer (only if the question actually asks for it):**

> One of our hosted applications was running slowly. I inspected the slow query logs and checked the execution plan with EXPLAIN. It showed a full table scan on a frequently used query. I put together the query, log entries, execution plan and my diagnostic notes for database engineering. They handled any change. I didn't implement a fix, and I never saw measurements that proved the performance improved.

**Possible deeper answer, when prompted for judgment:**

> I wanted to separate a database bottleneck from general server slowness before escalating. The execution plan and log pattern gave the database team something specific to verify, rather than a vague 'the site is slow' ticket. I wasn't the person authorized to change the production database, so I stopped at diagnosis and handoff.

**Unknowns which must stay unknown:** Which query change or index was ultimately applied; who approved it; whether performance improved and by how much; whether the user followed up with the customer.

### Earlier operations experience

Avery also coordinated billing and access escalations. They translated customer descriptions into actionable case notes and handed exceptions to the teams with authority to decide them. They did not own refund policy or engineering changes. These anecdotes may support a *hypothesis* about cross-functional diagnosis and handoff **only if actual interview records support it**.

### Demonstration discipline

Do not hardcode an output such as 'You are a systems engineer' or 'You saved the company money'. Realize should describe the narrower, defensible capability: structured investigation, evidence gathering, clear escalation and respect for ownership boundaries.
