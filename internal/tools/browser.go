package tools

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/chromedp"
)

// browserSession is a single Chrome window the agent controls. It uses its own
// persistent profile under the user's data directory, so sign-ins (Gmail,
// Outlook, ...) survive between runs. The window is visible so the user can
// log in themselves and watch what the agent does.
type browserSession struct {
	mu        sync.Mutex
	allocCtx  context.Context
	allocStop context.CancelFunc
	tabCtx    context.Context
	tabStop   context.CancelFunc
}

var browser = &browserSession{}

const maxPageChars = 9000

func browserProfileDir() string {
	dir := os.Getenv("XDG_DATA_HOME")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".local", "share")
	}
	return filepath.Join(dir, "localagent", "browser-profile")
}

// findChrome returns the first Chromium-based browser found on PATH.
func findChrome() (string, error) {
	for _, name := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "microsoft-edge", "brave-browser"} {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	return "", errors.New("no Chromium-based browser found (install Google Chrome or Chromium)")
}

// ensure starts Chrome if it is not running yet, or restarts it if the user closed it.
func (b *browserSession) ensure() (context.Context, error) {
	if b.tabCtx != nil && b.tabCtx.Err() == nil {
		return b.tabCtx, nil
	}
	b.closeLocked()
	exe, err := findChrome()
	if err != nil {
		return nil, err
	}
	profile := browserProfileDir()
	if err := os.MkdirAll(profile, 0o700); err != nil {
		return nil, err
	}
	opts := append([]chromedp.ExecAllocatorOption{},
		chromedp.ExecPath(exe),
		chromedp.UserDataDir(profile),
		chromedp.NoFirstRun,
		chromedp.NoDefaultBrowserCheck,
		chromedp.Flag("headless", false),
		chromedp.Flag("disable-blink-features", "AutomationControlled"),
		chromedp.Flag("disable-infobars", true),
		chromedp.Flag("window-size", "1280,900"),
	)
	b.allocCtx, b.allocStop = chromedp.NewExecAllocator(context.Background(), opts...)
	b.tabCtx, b.tabStop = chromedp.NewContext(b.allocCtx)
	// The browser process is bound to the context used for the first call, so
	// that call must use the long-lived tab context, not a timeout-derived one.
	done := make(chan error, 1)
	go func() { done <- chromedp.Do(b.tabCtx, chromedp.Navigate("about:blank")) }()
	select {
	case err := <-done:
		if err != nil {
			b.closeLocked()
			return nil, fmt.Errorf("could not start %s: %w", filepath.Base(exe), err)
		}
	case <-time.After(40 * time.Second):
		b.closeLocked()
		return nil, fmt.Errorf("%s did not start within 40s", filepath.Base(exe))
	}
	return b.tabCtx, nil
}

func (b *browserSession) closeLocked() {
	if b.tabStop != nil {
		b.tabStop()
	}
	if b.allocStop != nil {
		b.allocStop()
	}
	b.tabCtx, b.tabStop, b.allocCtx, b.allocStop = nil, nil, nil, nil
}

// CloseBrowser shuts the agent's browser window down (called on app exit).
func CloseBrowser() {
	browser.mu.Lock()
	defer browser.mu.Unlock()
	browser.closeLocked()
}

// exec runs fn against the browser tab with a timeout, restarting Chrome once
// if the user closed the window in between.
func (b *browserSession) exec(ctx context.Context, timeout time.Duration, fn func(ctx context.Context) error) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		tab, err := b.ensure()
		if err != nil {
			return err
		}
		runCtx, cancel := context.WithTimeout(tab, timeout)
		stop := context.AfterFunc(ctx, cancel)
		err = fn(runCtx)
		stop()
		cancel()
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		lastErr = err
		if tab.Err() != nil || strings.Contains(err.Error(), "websocket") || strings.Contains(err.Error(), "context canceled") {
			b.closeLocked() // browser went away; retry once with a fresh one
			continue
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("the page took too long to respond (%s)", timeout)
		}
		return err
	}
	return fmt.Errorf("browser window closed or crashed; please try again (%v)", lastErr)
}

// do runs a sequence of void actions.
func (b *browserSession) do(ctx context.Context, timeout time.Duration, steps ...chromedp.Action[chromedp.Void]) error {
	return b.exec(ctx, timeout, func(c context.Context) error { return chromedp.Do(c, steps...) })
}

// readPageJS extracts a compact, structured text view of the current page.
// Gmail and Outlook message lists get special handling so each row becomes one line.
const readPageJS = `(() => {
  const clean = s => (s || '').replace(/[ \t ]+/g, ' ').replace(/\s*\n\s*/g, '\n').replace(/\n{3,}/g, '\n\n').trim();
  const one = s => (s || '').replace(/\s+/g, ' ').trim();
  const host = location.hostname;
  const out = [];
  if (host === 'mail.google.com') {
    const rows = [...document.querySelectorAll('tr.zA')];
    if (rows.length) {
      out.push('Gmail message list, newest first (' + rows.length + ' shown). Times without a date are today.');
      rows.slice(0, 40).forEach((r, i) => {
        const from = one(r.querySelector('.yW')?.innerText);
        const subj = one(r.querySelector('.bog')?.innerText);
        const snip = one(r.querySelector('.y2')?.innerText).replace(/^[-–—]\s*/, '');
        const time = one(r.querySelector('.xW')?.innerText);
        const unread = r.classList.contains('zE') ? 'UNREAD' : 'read';
        out.push((i + 1) + '. [' + time + '] ' + unread + ' | from ' + from + ' | ' + subj + (snip ? ' | ' + snip : ''));
      });
      return out.join('\n');
    }
    const body = document.querySelector('.a3s');
    if (body) {
      const subj = one(document.querySelector('h2.hP')?.innerText);
      const from = one(document.querySelector('.gD')?.getAttribute('email') || document.querySelector('.gD')?.innerText);
      return 'Opened email\nSubject: ' + subj + '\nFrom: ' + from + '\n\n' + clean(body.innerText);
    }
  }
  if (/outlook\.(office|live|office365)\.com$/.test(host)) {
    const rows = [...document.querySelectorAll('[role="option"][aria-label]')];
    if (rows.length) {
      out.push('Outlook message list, newest first (' + rows.length + ' shown):');
      rows.slice(0, 40).forEach((r, i) => out.push((i + 1) + '. ' + one(r.getAttribute('aria-label'))));
      return out.join('\n');
    }
  }
  const main = document.querySelector('main, article, [role="main"]') || document.body;
  return clean(main ? main.innerText : '');
})()`

// clickJS finds the first clickable element whose visible text or label contains
// the query (case-insensitive, exact matches preferred) and clicks it.
const clickJS = `((query) => {
  const q = query.toLowerCase();
  const sel = 'a, button, [role="button"], [role="link"], [role="option"], [role="tab"], [role="menuitem"], [role="row"], tr.zA, input[type="submit"], label, summary, [onclick]';
  const els = [...document.querySelectorAll(sel)].filter(e => e.offsetParent !== null || e.tagName === 'TR');
  const text = e => ((e.innerText || e.value || '') + ' ' + (e.getAttribute('aria-label') || '') + ' ' + (e.getAttribute('title') || '')).replace(/\s+/g, ' ').trim().toLowerCase();
  let el = els.find(e => text(e) === q) || els.find(e => text(e).includes(q));
  if (!el) return 'No clickable element found containing "' + query + '"';
  el.scrollIntoView({block: 'center'});
  const target = el.matches('tr.zA') ? (el.querySelector('.bog') || el) : el;
  target.click();
  return 'clicked';
})`

// typeJS locates an input by CSS selector, placeholder, aria-label, name or
// nearby label text and tags it so chromedp can type into it.
const typeJS = `((query) => {
  const q = query.toLowerCase();
  document.querySelectorAll('[data-localagent-target]').forEach(e => e.removeAttribute('data-localagent-target'));
  let el = null;
  try { el = document.querySelector(query); } catch (_) {}
  const inputs = [...document.querySelectorAll('input, textarea, [contenteditable="true"], [role="textbox"], [role="combobox"]')].filter(e => e.offsetParent !== null);
  const attr = (e, a) => (e.getAttribute(a) || '').toLowerCase();
  if (!el) el = inputs.find(e => [attr(e,'placeholder'), attr(e,'aria-label'), attr(e,'name'), attr(e,'id'), attr(e,'title')].some(v => v && v.includes(q)));
  if (!el) {
    const lab = [...document.querySelectorAll('label')].find(l => l.innerText.toLowerCase().includes(q));
    if (lab) el = lab.control || (lab.htmlFor ? document.getElementById(lab.htmlFor) : lab.querySelector('input, textarea'));
  }
  if (!el && inputs.length === 1) el = inputs[0];
  if (!el) return 'No input found matching "' + query + '"';
  el.setAttribute('data-localagent-target', '1');
  el.scrollIntoView({block: 'center'});
  el.focus();
  if (el.value !== undefined) el.value = '';
  return 'ok';
})`

func isLoginPage(pageURL string) bool {
	u, err := url.Parse(pageURL)
	if err != nil {
		return false
	}
	h := strings.ToLower(u.Hostname())
	return strings.HasPrefix(h, "accounts.google.") || h == "login.microsoftonline.com" || h == "login.live.com" ||
		strings.Contains(h, "signin") || strings.Contains(h, "login.")
}

// snapshot reads title, URL and structured text of the current page.
func (b *browserSession) snapshot(ctx context.Context) (string, error) {
	var title, pageURL, text string
	err := b.exec(ctx, 20*time.Second, func(c context.Context) error {
		var err error
		if title, err = chromedp.Run(c, chromedp.Title()); err != nil {
			return err
		}
		if pageURL, err = chromedp.Run(c, chromedp.Location()); err != nil {
			return err
		}
		text, err = chromedp.Run(c, chromedp.Evaluate[string](readPageJS))
		return err
	})
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Page: %s\nURL: %s\n", title, pageURL)
	if isLoginPage(pageURL) {
		sb.WriteString("STATUS: this is a SIGN-IN page. The user must sign in themselves in the browser window that is open; never type passwords. Tell the user to sign in and ask again.\n")
	}
	sb.WriteString("\n")
	if strings.TrimSpace(text) == "" {
		sb.WriteString("(the page has no readable text yet; it may still be loading)")
	} else {
		sb.WriteString(truncate(text, maxPageChars))
	}
	return sb.String(), nil
}

func browserTools() []*Tool {
	return []*Tool{
		{
			Name: "browser_open",
			Description: "Open a web page in the agent's own browser window (a signed-in Chrome profile) and return the page's text. " +
				"Use this to READ web apps and sites: Gmail (https://mail.google.com/mail/u/0/#inbox), Outlook (https://outlook.office.com/mail/ or https://outlook.live.com/mail/), " +
				"calendars, news, documentation. If the result says SIGN-IN page, ask the user to sign in in that window and try again.",
			Parameters: schema(map[string]any{
				"url": prop("string", "Full URL to open, e.g. https://mail.google.com/mail/u/0/#inbox"),
			}, "url"),
			Classify: func(map[string]any) (Risk, string) { return Safe, "" },
			Run: func(ctx context.Context, args map[string]any) (string, error) {
				u := strings.TrimSpace(strArg(args, "url"))
				if u == "" {
					return "", fmt.Errorf("url is required")
				}
				if !strings.Contains(u, "://") {
					u = "https://" + u
				}
				if err := browser.do(ctx, 45*time.Second, chromedp.Navigate(u), chromedp.WaitReady("body"), chromedp.Sleep(3500*time.Millisecond)); err != nil {
					return "", err
				}
				return browser.snapshot(ctx)
			},
		},
		{
			Name:        "browser_read",
			Description: "Re-read the text of the page currently shown in the agent's browser window (after the user signed in, after a click, or after waiting for a page to load).",
			Parameters:  schema(map[string]any{}),
			Classify:    func(map[string]any) (Risk, string) { return Safe, "" },
			Run: func(ctx context.Context, args map[string]any) (string, error) {
				return browser.snapshot(ctx)
			},
		},
		{
			Name:        "browser_click",
			Description: "Click a link, button, tab, menu item or email row in the agent's browser by its visible text (for example an email subject to open it, 'Compose', 'Next', 'Sent'). Returns the page text after the click.",
			Parameters: schema(map[string]any{
				"text": prop("string", "Visible text or label of the element to click. Partial text is fine."),
			}, "text"),
			Classify: func(args map[string]any) (Risk, string) {
				return Risky, "clicks '" + strArg(args, "text") + "' in the browser"
			},
			Run: func(ctx context.Context, args map[string]any) (string, error) {
				text := strings.TrimSpace(strArg(args, "text"))
				if text == "" {
					return "", fmt.Errorf("text is required")
				}
				var result string
				err := browser.exec(ctx, 20*time.Second, func(c context.Context) error {
					var err error
					result, err = chromedp.Run(c, chromedp.Evaluate[string](fmt.Sprintf("%s(%q)", clickJS, text)))
					return err
				})
				if err != nil {
					return "", err
				}
				if result != "clicked" {
					return "", errors.New(result)
				}
				_ = browser.do(ctx, 10*time.Second, chromedp.Sleep(2500*time.Millisecond))
				return browser.snapshot(ctx)
			},
		},
		{
			Name:        "browser_type",
			Description: "Type text into an input field in the agent's browser, found by placeholder, label, name or CSS selector (for example 'Search mail', 'To', 'Subject'). Optionally press Enter afterwards. Never use this for passwords.",
			Parameters: schema(map[string]any{
				"field":       prop("string", "Placeholder, label, aria-label, name or CSS selector of the input."),
				"text":        prop("string", "Text to type."),
				"press_enter": prop("boolean", "Press Enter after typing (submits searches/forms)."),
			}, "field", "text"),
			Classify: func(args map[string]any) (Risk, string) {
				return Risky, "types into '" + strArg(args, "field") + "' in the browser"
			},
			Run: func(ctx context.Context, args map[string]any) (string, error) {
				field := strings.TrimSpace(strArg(args, "field"))
				text := strArg(args, "text")
				if field == "" {
					return "", fmt.Errorf("field is required")
				}
				lower := strings.ToLower(field)
				if strings.Contains(lower, "password") || strings.Contains(lower, "passwd") {
					return "", errors.New("refusing to type into a password field; the user must sign in themselves")
				}
				var result string
				err := browser.exec(ctx, 15*time.Second, func(c context.Context) error {
					var err error
					result, err = chromedp.Run(c, chromedp.Evaluate[string](fmt.Sprintf("%s(%q)", typeJS, field)))
					return err
				})
				if err != nil {
					return "", err
				}
				if result != "ok" {
					return "", errors.New(result)
				}
				keys := text
				if boolArg(args, "press_enter") {
					keys += "\r"
				}
				if err := browser.do(ctx, 20*time.Second, chromedp.SendKeys(`[data-localagent-target]`, keys), chromedp.Sleep(2*time.Second)); err != nil {
					return "", err
				}
				return browser.snapshot(ctx)
			},
		},
	}
}
