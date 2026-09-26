# resolve returns only an Application Target

`jobs-cli resolve` answers “where and how do I apply?”, not “what is the full Job?”. Its success `data` is an Application Target (canonical URL, Application Provider, identifiers, capabilities). Callers that still need Job fields use `jobs-cli show`. Mixing both into one default payload would blur command responsibility and encourage skipping explicit detail retrieval.
