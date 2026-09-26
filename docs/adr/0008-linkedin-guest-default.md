# LinkedIn defaults to Guest unless explicitly authenticated

`--source linkedin` uses Guest discovery by default. Voyager is used only when the caller passes an explicit authenticated switch (for example `--authenticated`). A present session must not silently upgrade the auth surface, rate-limit exposure, or failure modes of a search.
