# SDK Clean UP: 

Always start and validate the sdk loop on the go sdk. 
Then proceed with the others : c, c++, c#, dart, java, js, js-web, kotlin, python, ruby, rust, swift

## Prepare

Implement = OP build should implement generation from [template for Incode Description](../sdk/README.md#incode-description)


## For each SDK start by go. 

When go is done, proceed with the others taking GO as reference ( c, c++, c#, dart, java, js, js-web, kotlin, python, ruby, rust, swift)

1. Create the [template for Incode Description](../sdk/README.md#incode-description)
2. Fix = SDK silently fail to register their Describe endpoint make the error explicit.
3. Create the template for Incode Description
4. Create or verify a standard way to implement Describe, Identities and Discover.
5. Describe should work on isolated binary without .proto support. 
6. Analyze and control the [transport Matrix](../sdk/README.md#transport-matrix) using code review + tests to prove, no assumptions update it.
7. Implement the required transports for the SDK [based on the expected v0.6 transport Matrix](../sdk/README.md#expected-v06-transport-matrix), each implementation should be testable.
8. Check if all the other test in the SDK is working (if still relevent, remove irrelevant tests)
9. Use the SDK Incode Description and  rebuild using op the associated lang [example](../examples/hello-world/), fix anything required. 
10. Create for the SDK an accurate README.md as defined in [sdk/README.md](../sdk/README.md#per-sdk-documentation)

## When all SDK are done

Verify the [Swift UI Organism](../examples/hello-world/gabriel-greeting-app-swiftui)