# Test data

Files in this directory are deliberately small synthetic fixtures. They are not generated production datasets and may be committed safely.

`master/` is the canonical-only input shared by every domain generator test.
`expected/<domain>/` contains reviewed byte-for-byte golden output. Set
`UPDATE_GOLDEN=1` only when intentionally reviewing a domain contract change.
