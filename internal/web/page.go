package web

// The access credential stays in the URL fragment until this first-party script
// exchanges it for an HttpOnly session cookie. It never enters a query or log.
const bootstrapScript = `(async () => {
  const token = new URLSearchParams(location.hash.slice(1)).get('token');
  history.replaceState(null, '', location.pathname);
  const message = document.getElementById('status');
  try {
    if (token) {
      const response = await fetch('/api/session', {
        method: 'POST', headers: {'Content-Type': 'application/json'},
        body: JSON.stringify({token})
      });
      if (!response.ok) throw new Error('session');
    }
    const response = await fetch('/api/status');
    if (!response.ok) throw new Error('status');
    message.textContent = 'Connected to Tasky.';
  } catch {
    message.textContent = 'Open the Dashboard URL printed by tasky serve to connect.';
  }
})();`

const bootstrapPage = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Tasky</title></head>
<body><main><h1>Tasky</h1><p id="status" role="status">Connecting to the local service…</p></main><script>` + bootstrapScript + `</script></body></html>`
