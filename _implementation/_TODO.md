# General 
- drop mem:// support anywhere there is no real use case as the code needs to be running in one binary, we can use the public native api. shm:// would be interesting but is not currently realistic.

# Gabriel app swiftUI
- Simplify the code (no harcoded holon )
- support mem via the swift SDK

# OP

- I think that `op install` should not build the holon without explicit "--build" flag
- we need to review all the subcommand and provide a `op help <command>`
- Man of op should be integrated in the proto (and injected in the holon help) question à approfondir ... 