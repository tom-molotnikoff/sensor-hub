import { apiClient } from './client';

export function unknownQueryKeyFailsTheBuild() {
  return apiClient.GET('/readings/between', {
    params: {
      query: {
        start: '',
        end: '',
        // @ts-expect-error a query key not in the schema must fail the build
        measurement_type: '',
      },
    },
  });
}
