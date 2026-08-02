# Manual JSON inputs

Place manually maintained JSON payloads beneath this directory. The pipeline
discovers lowercase `.json` files recursively, derives each logical name from
its relative path without the extension, and publishes canonical copies under
the matching `manual/` release path.

Object keys are canonicalized deterministically. Array order is preserved.
