# Writing Telegram rich messages

This guide is for the model that calls `sendRichMessage` and `editMessage`. Read it before
sending the first rich message in a conversation. It covers when to send a rich message,
how to structure it, the syntax Telegram accepts (Bot API 10.5) and the limits.

## Choosing a tool

| Need | Tool |
|---|---|
| Report, summary, comparison, anything with structure | `sendRichMessage` with `markdown` |
| Buttons, compact or striped tables, cell alignment, merged cells | `sendRichMessage` with `html` |
| One-line ping that must render everywhere | `sendMessage` |
| Long logs, raw output, files | `sendDocument`, with a short rich message as the summary |
| Update a progress message in place | `editMessage` with the `message_id` you got back |

Prefer `markdown`. Use `html` only when a feature below is marked HTML-only. Never mix
the two fields: set exactly one.

Some Telegram Desktop versions show rich messages as empty. If the reader is likely on
desktop and the message is short and urgent, use `sendMessage` instead.

## What makes a good message

Telegram is read on a phone, often from a notification. Write for that reader.

1. **Lead with the outcome.** The first line is a heading or a bold sentence that answers
   "what happened?" (`## ✅ Deploy finished`, not `## Update`). Everything after is
   supporting detail.
2. **One idea per block.** Short paragraphs of 1 to 3 sentences. Use a list when you
   have 3 or more parallel items, and a table when items share 2 or more attributes.
3. **Shallow structure.** Use `##` for the title and `###` for sections; skip `#` (too
   large on phones) and `####` and deeper. Nest lists at most one level.
4. **Hide the bulk.** Put logs, stack traces, full file lists and secondary detail in
   `<details>`, so the message stays scannable while keeping everything.
5. **Narrow tables.** Phones fit about 3 or 4 columns. Keep cell text short, put the key
   column first, and use HTML `compact` for dense numeric tables. Transpose a table that
   has many columns and few rows.
6. **Emoji as markers, not decoration.** At most one per heading or status line, from a
   consistent set: ✅ success, ❌ failure, ⚠️ warning, ⏳ in progress, ℹ️ note.
7. **Code is code.** Use fenced blocks with a language (`` ```go ``) for code and
   commands, and inline `code` for identifiers, paths and values.
8. **Links over pasted URLs.** `[PR #42](https://...)` reads better than a bare URL.
9. **No filler.** No greetings, no "Here is your report", and no closing offers of help.

## Markdown syntax

Telegram rich Markdown is GitHub Flavored Markdown with extensions.

| Element | Syntax |
|---|---|
| Headings | `## Title`, `### Section` (levels 1 to 6) |
| Bold, italic | `**bold**`, `*italic*` |
| Strikethrough | `~~old~~` |
| Highlight | `==important==` |
| Spoiler | `\|\|hidden\|\|` |
| Inline code | `` `value` `` |
| Link | `[text](https://example.com)`; also `mailto:`, `tel:`, `tg://user?id=123` |
| Horizontal rule | `---` on its own line, with blank lines around it |
| Bullet, numbered list | `- item`, `1. item` |
| Task list | `- [ ] todo`, `- [x] done` |
| Quote | `> text` |
| Code block | fenced with three backticks and a language |
| Inline formula | `$x^2 + y^2$` (raw LaTeX) |
| Block formula | `$$E = mc^2$$`, or a fenced block with the `math` language |
| Footnote | `text[^1]`, then `[^1]: note` on its own line |
| Image | `![](https://example.com/chart.png "Caption")`, as its own paragraph |
| Local time | `![22:45 tomorrow](tg://time?unix=1647531900&format=wDT)` |

Tables use GFM pipe syntax; the delimiter row sets alignment:

```markdown
| Service | Status | p95 |
|:--------|:------:|----:|
| api     | ✅     | 120 ms |
| worker  | ⚠️     | 840 ms |
```

Cells take inline formatting only: no lists, code blocks or line breaks inside a cell.

Collapsible sections use HTML tags inside Markdown. Markdown is parsed inside
`<details>`, so keep blank lines around its content:

```markdown
<details><summary>Full log (212 lines)</summary>

    ...log lines...

</details>
```

Add `open` (`<details open>`) to show the section expanded by default.

Inside other HTML block tags, Markdown is not parsed. Escape a literal Markdown
character with a backslash, e.g. `\#not-a-hashtag` or `\*`.

### Time

`tg://time` and `<tg-time>` show a Unix timestamp in each reader's time zone. The fallback
text is shown where the format is unsupported. `format` is `r` (relative, e.g. "in 2
hours"), or any of `w` (weekday), then `d`/`D` (short/long date), then `t`/`T` (short/long
time), e.g. `wDT`, `dt`, `t`. Use it for deadlines and schedules instead of writing a time
zone by hand.

## HTML syntax

Use HTML for the features Markdown lacks. Escape `<`, `>` and `&` in text as `&lt;`,
`&gt;` and `&amp;`. Only these tags are supported:

- Inline: `<b>`, `<i>`, `<u>`, `<s>`, `<code>`, `<mark>`, `<sub>`, `<sup>`, `<tg-spoiler>`,
  `<a href>`, `<tg-math>`, `<tg-time unix format>`.
- Blocks: `<h1>`…`<h6>`, `<p>`, `<pre><code class="language-go">`, `<blockquote>`
  (add `expandable` to collapse a long quote), `<aside>` (pull quote), `<footer>`, `<hr/>`,
  `<tg-math-block>`, `<details>`/`<summary>`.
- Lists: `<ul>`, `<ol start type reversed>`, `<li>`, `<li><input type="checkbox" checked>…</li>`.
- Media: `<img src>`, `<video src>`, `<audio src>`, optionally in `<figure>` with
  `<figcaption>`.
- Tables: see below.
- Buttons: see below.

### Tables (HTML)

`<table>` takes the flags `bordered`, `striped` and `compact`, and `<caption>` as its first
child. Cells take `colspan`, `rowspan`, `align` (`left`, `center`, `right`) and `valign`
(`top`, `middle`, `bottom`):

```html
<table bordered striped compact>
<caption>Benchmarks</caption>
<tr><th>Case</th><th align="right">ns/op</th><th align="right">Δ</th></tr>
<tr><td>parse</td><td align="right">412</td><td align="right">−18%</td></tr>
<tr><td>encode</td><td align="right">97</td><td align="right">+2%</td></tr>
</table>
```

### Buttons (HTML)

Buttons can be placed in a row or inside a paragraph:

```html
<tg-button-row align="center">
<tg-button type="url" style="primary" url="https://github.com/org/repo/pull/42">Open PR</tg-button>
<tg-button type="copy_text" text="git checkout feature/login">Copy command</tg-button>
</tg-button-row>
```

- Types that work through this server: `url` (opens `url`), `copy_text` (copies `text`),
  `switch_inline_query` and `switch_inline_query_current_chat` (with `query`), and
  `disabled` (a visible but inactive label).
- `callback_data` buttons are rejected: nothing would answer the press.
- `style` is `primary`, `success` or `danger`; omit it for a neutral button.
- A row holds 1 to 8 buttons. Labels are plain text and should be 1 to 3 words.

## Limits

The server checks the first four before sending; Telegram enforces the rest.

| Limit | Value |
|---|---|
| Message length | 32768 characters, including formula source |
| Table width | 20 columns (about 4 readable on a phone) |
| Media | http(s) URLs or `tg://` links; local files go through `sendDocument` |
| Buttons | `url`, `copy_text`, `switch_inline_query*`, `disabled` only |
| Blocks | 500, counting nested blocks, list items and table rows |
| Nesting | 16 levels of blocks and formatting |
| Media per message | 50 |
| Buttons per row | 1 to 8 |

When a message would exceed a limit, split it at a heading into several messages, or
send the detail as a file with `sendDocument` and keep the rich message as the summary.

## Errors

A validation error from this server names the problem and the fix: apply it and retry.
A `Bad Request` from Telegram usually means unsupported syntax: an unknown HTML tag,
an unclosed tag, media inside a paragraph or a table cell, or a malformed table. Fix the
markup rather than falling back to plain text.

## Examples

### Task report

~~~markdown
## ✅ Migration to Postgres 17 finished

All 14 services run on the new cluster; no downtime.

### Results
| Step | Time | Status |
|:-----|-----:|:------:|
| Schema copy | 4 min | ✅ |
| Data sync | 38 min | ✅ |
| Cutover | 40 s | ✅ |

### Follow-ups
- [x] Switch read replicas
- [ ] Drop the old cluster after the next backup (`2026-12-01`)

<details><summary>Commands run</summary>

```sh
pg_upgrade --check
./cutover.sh --region eu
```

</details>
~~~

### Build failure

~~~markdown
## ❌ CI failed on `master`

`TestCheckout` fails after commit `a1b2c3d` ([run #812](https://ci.example.com/812)).

> expected status 200, got 500

**Likely cause:** the new price rounding in `cart/total.go:88`.

<details><summary>Test output (40 lines)</summary>

```text
--- FAIL: TestCheckout (0.31s)
    checkout_test.go:57: expected 200, got 500
```

</details>
~~~

### Comparison

```markdown
## Choosing a queue

**Recommendation:** NATS JetStream, which has the lowest operating cost at our volume.

| | NATS | Kafka | SQS |
|:--|:--:|:--:|:--:|
| Ops effort | Low | High | None |
| Ordering | Per subject | Per partition | FIFO queues |
| Cost / month | $40 | $310 | $95 |

The latency claim was measured on the staging cluster: $p_{95} \approx 3\,ms$.
```

### Progress message, edited in place

Send this first:

```markdown
## ⏳ Release v2.4.0

- [x] Tests
- [ ] Build images
- [ ] Deploy
```

Then call `editMessage` with the same `message_id` and the next state, ticking items off.
When the work is done, replace the heading with `## ✅ Release v2.4.0 is live` and add a link.
