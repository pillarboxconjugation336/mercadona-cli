'use strict';

// Best-effort: pre-fetch the platform binary at `npm install` time so the first run
// is instant. Never fail the install — if the download can't happen now (offline,
// --ignore-scripts, or a not-yet-published version), bin/run.js fetches it lazily on
// first invocation instead.

const { ensureBinary } = require('../lib/download.js');

ensureBinary().catch((e) => {
  process.stderr.write(`mercadona: postinstall download skipped (${e.message}).\n`);
  process.stderr.write('mercadona: the binary will be fetched automatically on first run.\n');
  // exit 0 on purpose — do not break `npm install`
});
