const express = require('express');
const app = express();
const port = process.env.PORT || 4000;

app.get('/', (req, res) => res.send('hello from sample-app'));
app.get('/healthz', (req, res) => res.status(200).send('ok'));

app.listen(port, () => console.log(`listening on ${port}`));
