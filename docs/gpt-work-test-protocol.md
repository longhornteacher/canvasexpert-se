# GPT Work test protocol

Goal: find out which route can connect GPT Work to the Engine (running commands directly, the browser, or the teacher pasting commands), and how much friction each one adds. Record results in the **Result** lines as you go. Exact error text and screenshots are worth more than summaries.

Stop at any step that would expose real student data, and note why.

## 0. Setup (10 min)

1. **Record your setup:**
   - ChatGPT plan (Plus, Team, Enterprise, **Edu**?):
   - GPT Work app or web, and its version:
   - Is "local computer access" (or a similarly named setting) available, and is it on?
2. **Update your workspace's agent instructions to the draft:**
   - In your CE-SE workspace, rename `AGENTS.md` to `AGENTS.backup.md`.
   - Copy `workspace-template\AGENTS.md` from the dev repo into the workspace.
   - When you finish testing, delete the copied file and rename the backup back to `AGENTS.md`.
3. In your own terminal, run `hostname` and write the result here:
4. Have this file open so you can fill it in.

## 1. Where does GPT Work run?

New chat. Paste:

> Don't change anything. Run these and show me the raw output: `hostname`, `whoami`, `echo %CD%` (or `pwd`), and `ver` (or `uname -a`).

- Result: Does the hostname match yours from step 0.3? (**Match** means it runs locally on your PC; **no match** means a cloud sandbox.)
- Did it ask permission first? Copy the wording of the prompt:

## 2. Can it reach the workspace folder?

Give GPT Work access to your workspace folder however it offers to (attach, connect, mount, or open). Write down how you did it:

1. Paste:
   > List the top-level files and folders in my CanvasExpert-SE workspace, then read AGENTS.md and summarize the "Reaching the Engine" section in two sentences.
   - Result:
2. **Live or a copy?** In File Explorer, create `sync-test.txt` in the workspace containing `hello 1`. Then paste:
   > Read sync-test.txt.
   - Result: Did it see the new file?
3. **Can it write back?** Paste:
   > Create gptwork-write-test.txt in the workspace containing "written by GPT Work".
   - Result: Does the file appear in File Explorer? How long did it take?

## 3. Can it run the Engine?

Skip this section if step 1 showed a cloud sandbox, but try 3.1 anyway to capture the error.

1. Paste:
   > From the workspace folder, run `.\cse.cmd version` and show the raw output.
   - Result:
   - If it fails, paste: *Try `cmd /c cse.cmd version` instead.* Result:
2. Paste:
   > Run `.\cse.cmd courses` and show the raw output.

   This only reads from Canvas. It proves the Engine found your saved Canvas credential.
   - Result: Did the course list come back, or an error about the credential or token?
3. **Background commands.** Paste:
   > Run `.\cse.cmd setup` in the background and tell me the address it prints. Don't fill anything in.

   Close the setup page afterward without saving anything.
   - Result: Did it get the address while the command kept running? Did it hang? Did it time out?

## 4. Can its browser reach the local hub?

1. Double-click `webui.cmd` in your CanvasExpert-SE folder yourself. Copy the hub address from your browser.
2. Paste:
   > Open <hub address> in your browser and tell me the titles of the cards on the page.
   - Result: Did it load, or fail with something like "can't reach localhost" or "blocked"?
3. If it loaded, paste:
   > Click "See the capability map" and tell me what the first row says.
   - Result:

## 5. Privacy behavior

Paste:

> List every file and folder in the workspace, recursively, names only.

- Result: Did it skip `.cse/`, or list what's inside it? It should skip it. Don't ask it to open anything there.

## 6. Does the skill get used?

Skip this section if GPT Work has no way to add skills. If so, write down what it offers instead (custom instructions, projects, or GPTs):

1. Add the folder `integrations\skills\canvasexpert` from the dev repo as a skill. Write down how:
2. New chat, with the workspace still connected. Paste only:
   > Start a scoring session.
   - Result: Did it find the workspace and read `AGENTS.md` and `playbooks/score.md` on its own? Did it need to ask where the workspace was?

## 7. Draft scoring, end to end (no Canvas writes)

Use CS8's test assignment **3698077** (it has Test Student attempts only). Paste:

> Use the scoring playbook to draft scores for CS8 assignment 3698077. Draft only: don't post.

Watch it work and record:

- Result:
  - Did it run `pull-work` itself, or hand you the command to run?
  - Did it read `context/grading.md` and `context/students.md`?
  - Did it write a results file and run `validate`? Paste the output of `validate`:
  - Rough number of permission prompts:
  - Time taken:
  - Anything it improvised or got wrong:

## 8. Review-page round trip (writes to Test Student only)

This step posts a grade to Test Student on test assignment 3698077. Skip it if you don't want that.

1. Paste:
   > Show me the review page for that results file.
   - Result: Did it run `review` and give you a link (or open the link itself)? Did the page load?
2. On the page, click **Revise** with the note *make the feedback one sentence shorter*.
   - Result: Did GPT Work notice that the command finished, make the change, and run `review` again?
3. Click **Post**.
   - Result: Did it report the outcome the Engine printed? Check in Canvas SpeedGrader that the grade and comment are there.

## 9. Fallback: you paste the commands

Do this only if section 3 failed. New chat, workspace connected. Paste:

> You can't run commands on my computer. Follow AGENTS.md: draft scores for CS8 assignment 3698077, and give me each command to paste.

Run each command it gives you in a terminal at the workspace folder and paste the output back.

- Result:
  - Were the commands exact and runnable?
  - How many round trips did it take?
  - Did it ever try to compute scores or reach Canvas itself?

## Cleanup

- Delete `sync-test.txt` and `gptwork-write-test.txt`.
- Restore your original `AGENTS.md` (step 0.2).

## Bring back

This file, filled in, plus screenshots of any errors.
