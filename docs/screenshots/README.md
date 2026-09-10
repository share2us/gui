# Screenshots

The real interface, captured from a production frontend build. Only the data is
invented: the Wails bridge is mocked so the window has devices, transfers and
activity to show.

Regenerate them after a UI change:

```sh
cd frontend && npm run build && cd ..
# serve frontend/dist with a mock window.go.main.App, then screenshot at
# 520x640 (the app's real window size) with deviceScaleFactor 2.
```

The harness lives in the planning repo under `store-screenshots/`; keep the
window size and the invented data consistent so the set stays coherent.
