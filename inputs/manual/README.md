# Manual JSON inputs

Place manually maintained JSON payloads beneath this directory. The pipeline
discovers lowercase `.json` files recursively, derives each logical name from
its relative path without the extension, and publishes compact canonical copies
under the matching `manual/` release path. Source files may remain formatted for
human maintenance; only the compact staged copies are published.

Object keys are canonicalized deterministically. Array order is preserved.
