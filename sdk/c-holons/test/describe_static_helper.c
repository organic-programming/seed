#include "holons/holons.h"

#include <stdio.h>
#include <string.h>

const holons_describe_response_t *holons_generated_describe_response(void);

int main(void) {
  char err[256];
  holons_describe_registration_t registration;
  holons_describe_response_t response;
  holons_describe_request_t request;
  const holons_service_doc_t *service;

  holons_use_static_describe_response(holons_generated_describe_response());

  err[0] = '\0';
  if (holons_make_describe_registration(&registration, err, sizeof(err)) != 0) {
    fprintf(stderr, "%s\n", err);
    return 1;
  }

  memset(&response, 0, sizeof(response));
  memset(&request, 0, sizeof(request));
  err[0] = '\0';
  if (holons_invoke_describe(&registration, &request, &response, err, sizeof(err)) != 0) {
    fprintf(stderr, "%s\n", err);
    return 1;
  }

  if (response.service_count != 1) {
    fprintf(stderr, "unexpected service count: %zu\n", response.service_count);
    holons_free_describe_response(&response);
    return 1;
  }

  service = &response.services[0];
  printf("slug=%s-%s\n",
         response.manifest.identity.given_name,
         response.manifest.identity.family_name);
  printf("service=%s\n", service->name);
  printf("methods=%zu\n", service->method_count);

  holons_free_describe_response(&response);
  holons_use_static_describe_response(NULL);
  return 0;
}
