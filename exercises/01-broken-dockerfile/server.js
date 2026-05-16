const express = require('express');

const app = express();

const policies = {
  'P-1001': { id: 'P-1001', holder: 'Alice Chen', premium: 1200, type: 'auto' },
  'P-1002': { id: 'P-1002', holder: 'Bob Diaz', premium: 800, type: 'home' },
  'P-1003': { id: 'P-1003', holder: 'Carla Singh', premium: 2400, type: 'life' },
};

app.get('/health', (_req, res) => {
  res.json({ status: 'ok', service: 'policy-api' });
});

app.get('/policies/:id', (req, res) => {
  const p = policies[req.params.id];
  if (!p) return res.status(404).json({ error: 'not found' });
  res.json(p);
});

if (require.main === module) {
  const port = process.env.PORT || 3000;
  app.listen(port, () => console.log(`policy-api listening on ${port}`));
}

module.exports = app;
