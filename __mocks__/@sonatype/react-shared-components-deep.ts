// Generic deep mock for @sonatype/react-shared-components subpath imports

// For validationUtil
export const hasValidationErrors = (errors: unknown): boolean =>
  errors !== null && errors !== undefined;

// For NxTextInput/types
export type StateProps = {
  value: string;
  trimmedValue: string;
  isPristine: boolean;
  validationErrors: string | null | string[];
};

export type Validator = (val: string) => string | null;
