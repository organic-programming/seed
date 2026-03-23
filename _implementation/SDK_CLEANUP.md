# SDK Clean UP: 

Always start and validate the sdk loop on the go sdk. 
Then proceed with the others : c, c++, c#, dart, java, js, js-web, kotlin, python, ruby, rust, swift

## For each SDK 

1. "op build" should create [template for Incode Description](../sdk/README.md#incode-description)
2. Fix = SDK silently fail to register their Describe endpoint make the error explicit.
3. Create the template for Incode Description
4. Create or verify a standard way to implement Describe, Identities and Discover.
5. Describe should work on isolated binary without .proto support. 
6. Rebuild the associated lang [example](../examples/hello-world/)