Combine the previous modules into:

the Advertising Autonomous Analyst.

User input:

"Why did Meta ad ROI decline yesterday"

The agent automatically:

1. Understands the goal
2. Identifies:
   channel=Meta
   metric=ROAS
   time=yesterday
3. Fetches:
   spend
   revenue
   impressions
   clicks
   conversions
4. Calculates:
   ROAS
   CTR
   CPC
   CVR
   CPA
5. Compares:
   yesterday
   previous day
   7 day average
6. Finds anomalous campaigns
7. Finds anomalous ad groups
8. Finds anomalous creatives
9. Finds anomalous countries
10. Continues querying when needed
11. Forms a root cause
12. Has the Reviewer verify it
13. Returns the final report.

Output shape:

{
  "summary": "",
  "metrics": {},
  "anomalies": [],
  "root_causes": [],
  "evidence": [],
  "recommendations": [],
  "confidence": 0.0
}

Important:

The agent must decide on its own whether to keep querying.

Example:

If ROAS dropped:

do not finish immediately.

Keep checking:

Did revenue drop?
Did spend rise?
Did CTR drop?
Did CPC rise?
Did CVR drop?
Is a campaign anomalous?
Is a creative anomalous?

Finish only when:

EvidenceSufficient = true

Every conclusion must cite evidence.

Do not generate a cause without evidence.

Add an end-to-end integration test.
