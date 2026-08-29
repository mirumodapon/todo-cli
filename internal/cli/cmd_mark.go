package cli

import "fmt"

func (a *App) cmdDone(args []string) error   { return a.setDone(args, true) }
func (a *App) cmdUndone(args []string) error { return a.setDone(args, false) }

func (a *App) setDone(args []string, done bool) error {
	ids, err := parseIDs(args)
	if err != nil {
		return err
	}
	verb := "已完成"
	if !done {
		verb = "已取消完成"
	}
	for _, id := range ids {
		t, err := a.Store.Get(id)
		if err != nil {
			return fmt.Errorf("#%d：%w", id, err)
		}
		if err := a.Store.SetDone(id, done, a.Now()); err != nil {
			return fmt.Errorf("#%d：%w", id, err)
		}
		fmt.Fprintf(a.Out, "%s #%d：%s\n", verb, id, t.Title)
	}
	return nil
}

func (a *App) cmdRm(args []string) error {
	ids, err := parseIDs(args)
	if err != nil {
		return err
	}
	for _, id := range ids {
		// 先取回來，刪除訊息才能帶上標題，讓使用者確認自己刪對了。
		t, err := a.Store.Get(id)
		if err != nil {
			return fmt.Errorf("#%d：%w", id, err)
		}
		if err := a.Store.Delete(id); err != nil {
			return fmt.Errorf("#%d：%w", id, err)
		}
		fmt.Fprintf(a.Out, "已刪除 #%d：%s\n", id, t.Title)
	}
	return nil
}
