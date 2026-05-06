/*
 * Copyright (c) 2021-present Sonatype, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
import { NxButton, NxCheckbox, NxFieldset, NxFormGroup, NxLoadError, NxTextInput, nxTextInputStateHelpers, NxTooltip, useToggle } from "@sonatype/react-shared-components";
import React, { FormEvent, useState, useTransition } from "react";
import classnames from 'classnames';
import { none } from 'ramda';
import { hasValidationErrors } from '@sonatype/react-shared-components/util/validationUtil';
import CLABody from "../ClaBody/CLABody";
import { StateProps, Validator } from "@sonatype/react-shared-components/components/NxTextInput/types";
import './Body.css';

type GitHubUser = {
  login: string
  email?: string
  name?: string
}

type SignCla = {
  user: GitHubUser
  claVersion: string
  claTextUrl: string
}

type QueryError = {
  error: boolean
  errorMessage: string
}

type StatePropsSetter = (state: StateProps) => void;

type ScrollEvent = {
  target: {
    scrollTop: number;
    clientHeight: number;
    scrollHeight: number;
  };
};

const handleScroll = (event: ScrollEvent, setScrolled: (scrolled: boolean) => void) => {
  const el = event.target;
  if (Math.round(el.scrollTop + el.clientHeight) === el.scrollHeight) {
    setScrolled(true);
  }
};

const hasCode = (url: string): boolean => {
  return url.startsWith("?code=");
}

const { initialState, userInput } = nxTextInputStateHelpers;

const Body = () => {

    const [isPending, startTransition] = useTransition();

    const validator = (val: string) => {
      return val.length ? null : 'Must be non empty';
    }

    const [loggedIn, setLoggedIn] = useState(false),
          [scrolled, setScrolled] = useState(false),
          [username, setUsername] = useState(initialState('', validator)),
          [ghState, setGHState] = useState<string>(""),
          [email, setEmail] = useState(initialState('', validator)),
          [fullName, setFullName] = useState(initialState('', validator)),
          [user, setUser] = useState<GitHubUser | undefined>(undefined),
          [queryError, setQueryError] = useState<QueryError>({error: false, errorMessage: ""}),
          [isOpen, dismiss] = useToggle(true),
          [agreeToTerms, setAgreeToTerms] = useState(false);

    const stateHasValidationErrors = (state: StateProps) => hasValidationErrors(state.validationErrors),
          isValid = none(stateHasValidationErrors, [username, email, fullName]),
          hasAllRequiredData = !!(email.trimmedValue && fullName.trimmedValue && scrolled && agreeToTerms && loggedIn),
          isSubmittable = isValid && hasAllRequiredData;

    const nonEmptyValidator = (val: string) => val && val.length ? null : 'Must be non-empty';

    const setTextInput = (setter: StatePropsSetter, validator?: Validator) => (value: string) => {
      setter(userInput(validator, value));
    };

    const getGitHubAuthUrl = (): string => {
      const urlParams = new URLSearchParams(window.location.search);

      const originalUri = urlParams.get("original_uri");

      const state: string = (originalUri) ? originalUri : process.env.REACT_APP_COMPANY_WEBSITE;

      const currentUrl = window.location.href.split('?')[0];

      return `https://github.com/login/oauth/authorize?client_id=${process.env.REACT_APP_GITHUB_CLIENT_ID}&redirect_uri=${currentUrl}&scope=user:email&state=${state}`;
    }

    const getUser = async (search: string) => {
      if (!user && !loggedIn) {
        const urlParams = new URLSearchParams(search);

        const code = urlParams.get("code");
        const redirectState = urlParams.get("state");

        const params = new URLSearchParams({ code: code ?? '', state: redirectState ?? '' });
        const res = await fetch(`/oauth-callback?${params}`);
        if (!res.ok) {
          const msg = await res.text();
          setQueryError({ error: true, errorMessage: msg });
          return;
        }
        const githubUser: GitHubUser = await res.json();

        setUser(githubUser);
        setLoggedIn(true);
        setGHState(redirectState!);

        setUsername({value: githubUser.login, trimmedValue: githubUser.login.trim(), isPristine: true});
        setEmail(
          (githubUser.email)
            ? {value: githubUser.email, trimmedValue: githubUser.email.trim(), isPristine: true}
            : {value: "", trimmedValue: "", isPristine: true}
        );
        setFullName(
          (githubUser.name)
            ? {value: githubUser.name, trimmedValue: githubUser.name.trim(), isPristine: true}
            : {value: "", trimmedValue: "", isPristine: true}
        );
      }
    }

    const doSubmit = (evt: FormEvent) => {
      evt.preventDefault();

      if (isSubmittable) {
        const signUser: SignCla = {
          user: {
            login: user!.login,
            email: email.value,
            name: fullName.value
          },
          claVersion: (process.env.REACT_APP_CLA_VERSION) ? process.env.REACT_APP_CLA_VERSION : "",
          claTextUrl: (process.env.REACT_APP_CLA_URL) ? process.env.REACT_APP_CLA_URL : ""
        };

        startTransition(async () => {
          const res = await fetch('/sign-cla', {
            method: 'PUT',
            headers: { Accept: 'application/json', 'Content-Type': 'application/json' },
            body: JSON.stringify(signUser),
          });
          if (!res.ok) {
            const msg = await res.text();
            setQueryError({ error: true, errorMessage: msg });
            return;
          }
          const redirectTarget = decodeURIComponent(ghState);
          try {
            const { protocol } = new URL(redirectTarget);
            if (protocol !== 'https:') {
              setQueryError({ error: true, errorMessage: 'Invalid redirect destination' });
              return;
            }
          } catch {
            setQueryError({ error: true, errorMessage: 'Invalid redirect destination' });
            return;
          }
          window.location.assign(redirectTarget);
        });
      } else {
        evt.stopPropagation();
      }
    }

    const submitBtnClasses = classnames({ disabled: !isSubmittable }),
      submitTooltip = isSubmittable ? '' :
      hasAllRequiredData ? 'Validation errors are present' :
      'Required fields are missing';

    const doRender = () => {

      if (hasCode(window.location.search) && !loggedIn) {
        getUser(window.location.search);
      }

      if (queryError.error) {
        return isOpen ? <NxLoadError error={queryError.errorMessage} onClose={dismiss}/> : null;
      }

      return <React.Fragment>

        <h1>Sign the {process.env.REACT_APP_COMPANY_NAME} Contributor License Agreement (CLA)</h1>

        <NxCheckbox
          checkboxId="login-check"
          isChecked={loggedIn}
          disabled={true}>
          Authenticate with GitHub so we can associate your commits with your signed CLA
        </NxCheckbox>

        { !loggedIn && (
          <a href={getGitHubAuthUrl()} className="nx-btn nx-btn--primary">Login to Github</a>
        )}

        { loggedIn && user && (
          <h3>Logged in as: { user.login }</h3>
        )}

        <NxCheckbox
          checkboxId="cla-check"
          isChecked={scrolled}
          disabled={true}>
          Review the CLA version: {process.env.REACT_APP_CLA_VERSION}
        </NxCheckbox>

        <CLABody
          handleScroll={(e) =>
            handleScroll({ target: e.target as unknown as { scrollTop: number; clientHeight: number; scrollHeight: number } }, setScrolled)
          }/>

        <NxCheckbox
          checkboxId="sign-cla-check"
          isChecked={agreeToTerms}
          disabled={true}>
          I agree to the terms of CLA version {process.env.REACT_APP_CLA_VERSION}
        </NxCheckbox>

        { !loggedIn && (
          <a href={getGitHubAuthUrl()} className="nx-btn nx-btn--primary">Login via Github to sign the CLA</a>
        )}

        { loggedIn && user && (
          <form className="nx-form" onSubmit={doSubmit}>

            <NxFormGroup
              label="Username"
              isRequired={true}>
              <NxTextInput
                disabled={true}
                validatable={true}

                value={username.value}
                isPristine={username.isPristine}
              />
            </NxFormGroup>

            <NxFormGroup
              label="Email Address"
              isRequired={true}>
              <NxTextInput
                onChange={setTextInput(setEmail, nonEmptyValidator)}
                validatable={true}

                value={email.value}
                isPristine={email.isPristine}
              />
            </NxFormGroup>

            <NxFormGroup
              label="Full Name"
              isRequired={true}>
              <NxTextInput
                onChange={setTextInput(setFullName, nonEmptyValidator)}
                validatable={true}

                value={fullName.value}
                isPristine={fullName.isPristine}
              />
            </NxFormGroup>

            <NxFieldset
              label="I agree to the terms of the above CLA"
              isRequired={true}>

              <NxCheckbox
                checkboxId="sign-cla-check"
                isChecked={agreeToTerms}
                onChange={() => setAgreeToTerms(true)}>
                Yes
              </NxCheckbox>

            </NxFieldset>

            <footer className="nx-form-footer">
              <div className="nx-btn-bar">
                <NxTooltip title={submitTooltip}>
                  <NxButton
                    className={submitBtnClasses}
                    variant="primary"
                    type="submit"
                    disabled={!isSubmittable || isPending}>
                    Sign the CLA
                  </NxButton>
                </NxTooltip>
              </div>
            </footer>

          </form>
        )}

        </React.Fragment>
    }

    return (
      doRender()
    )
}

export default Body;

export { handleScroll, hasCode }
