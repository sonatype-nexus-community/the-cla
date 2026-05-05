import React from 'react';

// Mock for @sonatype/react-shared-components in tests

export const NxButton = ({ children, ...props }: React.PropsWithChildren<React.ButtonHTMLAttributes<HTMLButtonElement>>) =>
  <button {...props}>{children}</button>;

export const NxCheckbox = ({ children, isChecked, disabled, onChange, checkboxId }: {
  children?: React.ReactNode;
  isChecked: boolean;
  disabled?: boolean;
  onChange?: () => void;
  checkboxId: string;
}) =>
  <label htmlFor={checkboxId}><input id={checkboxId} type="checkbox" checked={isChecked} disabled={disabled} onChange={onChange ?? (() => {})} />{children}</label>;

export const NxFieldset = ({ children, label }: { children?: React.ReactNode; label?: string; isRequired?: boolean }) =>
  <fieldset><legend>{label}</legend>{children}</fieldset>;

export const NxFormGroup = ({ children, label }: { children?: React.ReactNode; label?: string; isRequired?: boolean }) =>
  <div><label>{label}</label>{children}</div>;

export const NxLoadError = ({ error, onClose }: { error: string; onClose?: () => void }) =>
  <div role="alert">{error}<button onClick={onClose}>Close</button></div>;

export const NxLoadingSpinner = () => <div role="status">Loading...</div>;

export const NxTextInput = ({ value, onChange, disabled, validatable, isPristine }: {
  value?: string;
  onChange?: (val: string) => void;
  disabled?: boolean;
  validatable?: boolean;
  isPristine?: boolean;
}) =>
  <input value={value ?? ''} disabled={disabled} onChange={e => onChange?.(e.target.value)} />;

export const NxTooltip = ({ children, title }: { children?: React.ReactNode; title?: string }) =>
  <>{children}</>;

export const NxPageHeader = ({ productInfo, className }: { productInfo?: { name?: string; version?: string }; className?: string }) =>
  <header className={className}>{productInfo?.name} {productInfo?.version}</header>;

export const nxTextInputStateHelpers = {
  initialState: (value: string, validator?: (val: string) => string | null) => ({
    value,
    trimmedValue: value.trim(),
    isPristine: true,
    validationErrors: validator ? validator(value) : null,
  }),
  userInput: (validator: ((val: string) => string | null) | undefined, value: string) => ({
    value,
    trimmedValue: value.trim(),
    isPristine: false,
    validationErrors: validator ? validator(value) : null,
  }),
};

export const useToggle = (initialValue: boolean): [boolean, () => void] => {
  const [state, setState] = React.useState(initialValue);
  const toggle = () => setState(s => !s);
  return [state, toggle];
};

export const hasValidationErrors = (errors: unknown) => errors !== null && errors !== undefined;
