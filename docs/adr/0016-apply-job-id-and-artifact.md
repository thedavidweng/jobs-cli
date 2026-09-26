# apply entrypoints take a Job ID; submit consumes an Application Artifact

`apply inspect` and `apply prepare` accept a Job ID and resolve the Application Target as needed. `apply submit` consumes a previously prepared Application Artifact (file or stdin), not a raw Job ID alone. That keeps the inspect → prepare → submit handoff explicit for Agents and humans.
