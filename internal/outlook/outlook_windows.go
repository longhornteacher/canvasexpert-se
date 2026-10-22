//go:build windows

package outlook

import (
	"errors"
	"fmt"
	"runtime"
	"strings"

	ole "github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// All COM work runs on one OS-locked goroutine fed over a channel.
type request struct {
	run  func(app *ole.IDispatch) (any, error)
	done chan result
}
type result struct {
	v   any
	err error
}

var reqs = make(chan request)

func init() { go worker() }

func worker() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		for r := range reqs {
			r.done <- result{err: explain(err)}
		}
		return
	}
	defer ole.CoUninitialize()
	for r := range reqs {
		r.done <- call(r.run)
	}
}

func call(run func(*ole.IDispatch) (any, error)) result {
	unk, err := oleutil.GetActiveObject("Outlook.Application")
	if err != nil {
		unk, err = oleutil.CreateObject("Outlook.Application")
		if err != nil {
			return result{err: explain(err)}
		}
	}
	defer unk.Release()
	app, err := unk.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return result{err: explain(err)}
	}
	defer app.Release()
	v, err := run(app)
	return result{v, err}
}

func do(run func(*ole.IDispatch) (any, error)) (any, error) {
	r := request{run: run, done: make(chan result, 1)}
	reqs <- r
	res := <-r.done
	return res.v, res.err
}

// explain maps COM failures to a plain reason.
func explain(err error) error {
	s := strings.ToLower(err.Error())
	var reason string
	switch {
	case strings.Contains(s, "invalid class string"), strings.Contains(s, "class not registered"), strings.Contains(s, "80040154"):
		reason = "classic Outlook is not installed (new Outlook alone cannot be automated)"
	case strings.Contains(s, "80010108"), strings.Contains(s, "800706ba"), strings.Contains(s, "80080005"), strings.Contains(s, "access is denied"), strings.Contains(s, "80070005"):
		reason = "Outlook could not be reached. If CE-SE or Outlook was started as administrator, restart both normally"
	case strings.Contains(s, "operation aborted"), strings.Contains(s, "80004004"):
		reason = "Outlook refused the request (a security prompt was declined or the object model is guarded)"
	case strings.Contains(s, "profile"), strings.Contains(s, "mapi"):
		reason = "Outlook has no usable mail profile"
	default:
		reason = err.Error()
	}
	return fmt.Errorf("%w: %s", ErrUnavailable, reason)
}

// Status reports whether classic Outlook can be driven right now.
func Status() (bool, string) {
	_, err := do(func(app *ole.IDispatch) (any, error) {
		v, err := oleutil.GetProperty(app, "Version")
		if err != nil {
			return nil, err
		}
		v.Clear()
		return nil, nil
	})
	if err != nil {
		return false, strings.TrimPrefix(err.Error(), ErrUnavailable.Error()+": ")
	}
	return true, ""
}

func compose(app *ole.IDispatch, m Message) (*ole.IDispatch, error) {
	v, err := oleutil.CallMethod(app, "CreateItem", 0) // olMailItem
	if err != nil {
		return nil, err
	}
	item := v.ToIDispatch()
	fail := func(err error) (*ole.IDispatch, error) {
		item.Release()
		return nil, err
	}
	if _, err := oleutil.PutProperty(item, "To", strings.Join(m.To, ";")); err != nil {
		return fail(err)
	}
	if len(m.Cc) > 0 {
		if _, err := oleutil.PutProperty(item, "CC", strings.Join(m.Cc, ";")); err != nil {
			return fail(err)
		}
	}
	if len(m.ReplyTo) > 0 {
		rv, err := oleutil.GetProperty(item, "ReplyRecipients")
		if err != nil {
			return fail(err)
		}
		reply := rv.ToIDispatch()
		defer reply.Release()
		for _, addr := range m.ReplyTo {
			r, err := oleutil.CallMethod(reply, "Add", addr)
			if err != nil {
				return fail(err)
			}
			r.ToIDispatch().Release()
		}
		ok, err := oleutil.CallMethod(reply, "ResolveAll")
		if err != nil {
			return fail(err)
		}
		if b, _ := ok.Value().(bool); !b {
			return fail(errors.New("a Reply-To address did not resolve"))
		}
	}
	if _, err := oleutil.PutProperty(item, "Subject", m.Subject); err != nil {
		return fail(err)
	}
	if m.HTMLBody != "" {
		_, err = oleutil.PutProperty(item, "HTMLBody", m.HTMLBody)
	} else {
		_, err = oleutil.PutProperty(item, "Body", m.TextBody)
	}
	if err != nil {
		return fail(err)
	}
	if len(m.Attachments) > 0 {
		// Outlook requires a saved item before reliably changing Attachments.
		// This is always this newly created item, never an existing saved draft.
		if _, err := oleutil.CallMethod(item, "Save"); err != nil {
			return fail(err)
		}
		av, err := oleutil.GetProperty(item, "Attachments")
		if err != nil {
			return fail(err)
		}
		attachments := av.ToIDispatch()
		defer attachments.Release()
		for _, a := range m.Attachments {
			if err := addAttachment(attachments, a); err != nil {
				return fail(err)
			}
		}
	}
	// Every recipient must resolve before anything is sent.
	recv, err := oleutil.GetProperty(item, "Recipients")
	if err != nil {
		return fail(err)
	}
	rec := recv.ToIDispatch()
	defer rec.Release()
	ok, err := oleutil.CallMethod(rec, "ResolveAll")
	if err != nil {
		return fail(err)
	}
	if b, _ := ok.Value().(bool); !b {
		return fail(errors.New("a recipient address did not resolve"))
	}
	return item, nil
}

func addAttachment(attachments *ole.IDispatch, a Attachment) error {
	// Omit Position: zero hides an attachment in Outlook. Inline images are
	// hidden explicitly by their CID metadata; ordinary files stay visible.
	v, err := oleutil.CallMethod(attachments, "Add", a.Path, 1) // olByValue
	if err != nil {
		return attachmentFailure("add a local attachment", err)
	}
	attachment := v.ToIDispatch()
	defer attachment.Release()
	if a.ContentID == "" && a.ContentType == "" {
		return nil
	}
	pv, err := oleutil.GetProperty(attachment, "PropertyAccessor")
	if err != nil {
		return attachmentFailure("access attachment properties", err)
	}
	accessor := pv.ToIDispatch()
	defer accessor.Release()
	properties := attachmentProperties(a)
	for _, property := range properties {
		if _, err := oleutil.CallMethod(accessor, "SetProperty", property.Schema, property.Value); err != nil {
			return attachmentFailure("set attachment property "+strings.TrimPrefix(property.Schema, "http://schemas.microsoft.com/mapi/proptag/"), err)
		}
	}
	return nil
}

// Keep diagnostic stages and COM codes useful without returning Outlook's raw
// error text, which can include a private local path or recipient address.
func attachmentFailure(stage string, err error) error {
	var comError *ole.OleError
	if errors.As(err, &comError) {
		return fmt.Errorf("Outlook could not %s (COM 0x%08X)", stage, comError.Code())
	}
	return fmt.Errorf("Outlook could not %s", stage)
}

// Send creates a fresh MailItem and sends it. An error after the Send call
// began is reported as Uncertain, since Outlook may have accepted it.
func Send(m Message) (Outcome, error) {
	if err := validate(m); err != nil {
		return Failed, err
	}
	out := Failed
	_, err := do(func(app *ole.IDispatch) (any, error) {
		item, err := compose(app, m)
		if err != nil {
			return nil, err
		}
		defer item.Release()
		out = Uncertain
		if _, err := oleutil.CallMethod(item, "Send"); err != nil {
			return nil, err
		}
		out = Sent
		return nil, nil
	})
	if err != nil {
		if out == Failed {
			return Failed, explainIfCOM(err)
		}
		return out, err
	}
	return out, nil
}

// SaveDraft saves a fresh item to Drafts without sending it.
func SaveDraft(m Message) error {
	if err := validate(m); err != nil {
		return err
	}
	_, err := do(func(app *ole.IDispatch) (any, error) {
		item, err := compose(app, m)
		if err != nil {
			return nil, err
		}
		defer item.Release()
		_, err = oleutil.CallMethod(item, "Save")
		return nil, err
	})
	return explainIfCOM(err)
}

func explainIfCOM(err error) error {
	if err == nil || errors.Is(err, ErrUnavailable) {
		return err
	}
	var oe *ole.OleError
	if errors.As(err, &oe) {
		return explain(err)
	}
	return err
}

// SelfAddress returns the signed-in profile's own SMTP address, for the
// self-test and the "send a test to me" button only.
func SelfAddress() (string, error) {
	v, err := do(func(app *ole.IDispatch) (any, error) {
		sv, err := oleutil.GetProperty(app, "Session")
		if err != nil {
			return nil, err
		}
		sess := sv.ToIDispatch()
		defer sess.Release()
		uv, err := oleutil.GetProperty(sess, "CurrentUser")
		if err != nil {
			return nil, err
		}
		user := uv.ToIDispatch()
		defer user.Release()
		ev, err := oleutil.GetProperty(user, "AddressEntry")
		if err != nil {
			return nil, err
		}
		entry := ev.ToIDispatch()
		defer entry.Release()
		if ex, err := oleutil.CallMethod(entry, "GetExchangeUser"); err == nil {
			if d := ex.ToIDispatch(); d != nil {
				defer d.Release()
				if a, err := oleutil.GetProperty(d, "PrimarySmtpAddress"); err == nil && a.ToString() != "" {
					return a.ToString(), nil
				}
			}
		}
		a, err := oleutil.GetProperty(user, "Address")
		if err != nil {
			return nil, err
		}
		return a.ToString(), nil
	})
	if err != nil {
		return "", explainIfCOM(err)
	}
	s, _ := v.(string)
	if s == "" {
		return "", errors.New("the Outlook profile has no address")
	}
	return s, nil
}
