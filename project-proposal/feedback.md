# Professor feedback on project proposal

Mapped to `II2202-proposal.tex`.

---

## 1. Citations — be more specific on repeated uses

**Where:** Related work, around lines 73–76 (and earlier cites at lines 70, 83).

**Passage:**

> Prior work has looked at this problem from several angles. Jensen, Howard, and Mortier [1] …

**Feedback:** When the same paper is cited more than once, make citations more specific (page, section, equation, figure, etc.) instead of only a bare `[n]`.

---

## 2. Citation style — author names vs. bracket-only

**Where:** Line 74.

**Passage:**

> … they do not implement or measure that approach. Alfatafta et al. introduced Nifty …

**Feedback:** Prefer either:

- `Alfatafta et al. [2] introduced Nifty …`, or
- `[2] introduced Nifty …`

Avoid naming the authors without attaching the citation number in the same phrase.

---

## 3. RQ1 — make the topology set explicit

**Where:** Line 94 (RQ1); topologies are listed later in Table 1 / lines 117–145.

**Passage:**

> RQ1 (Liveness): Under which partial-connectivity topologies …

**Feedback:** State clearly that the experiment will be run on a defined set of different topologies (not only implied by the later table).

---

## 4. Experimental design — sample size / how many runs

**Where:** Lines 104–105 (and related procedure text at lines 151–153).

**Passage:**

> … introduce artificial delays. Our intention is to run each identified topology twice: once with ordinary point-to-point communication (without any overlay), and once with transparent multi-hop forwarding.

**Feedback:** How many samples will you collect? Clarify total runs / samples beyond “twice per topology.”

---

## 5. Evaluation matrix — repetitions and statistics

**Where:** Table 2 (`tab:matrix`), lines 163–177; analysis text at lines 153, 157–159.

**Passage:** The “Repetitions” column (currently 5).

**Feedback:**

- Is five repetitions enough?
- Which statistical analysis will you perform?

---

## 6. References — coverage

**Where:** Bibliography at end (lines 181–182 / `.bib` file).

**Feedback:** Are there any more references that should be included?
