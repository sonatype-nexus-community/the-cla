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
import React, { FormEvent, useContext, useState } from "react";
import { Action, ClientContext } from "react-fetching-library";
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

type queryError = {
  error: boolean
  errorMessage: string
}

type StatePropsSetter = (state: StateProps) => void;

const handleScroll = (event: any, setScrolled: (scrolled: boolean) => any) => {
  let el = event.target;
  if (Math.round(el.scrollTop + el.clientHeight) === el.scrollHeight) {
    setScrolled(true);
  }
};

const hasCode = (url: string): boolean => {
  return url.startsWith("?code=");
}

const { initialState, userInput } = nxTextInputStateHelpers;

const Body = () => {

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
          [queryError, setQueryError] = useState<queryError>({error: false, errorMessage: ""}),
          [isOpen, dismiss] = useToggle(true),
          [agreeToTerms, setAgreeToTerms] = useState(false);

    const stateHasValidationErrors = (state: StateProps) => hasValidationErrors(state.validationErrors),
          isValid = none(stateHasValidationErrors, [username, email, fullName]),
          hasAllRequiredData = !!(email.trimmedValue && fullName.trimmedValue && scrolled && agreeToTerms && loggedIn),
          isSubmittable = isValid && hasAllRequiredData;

    const nonEmptyValidator = (val: string) => val && val.length ? null : 'Must be non-empty';

    const clientContext = useContext(ClientContext);

    const setTextInput = (setter: StatePropsSetter, validator?: Validator) => (value: string) => {
      setter(userInput(validator, value));
    };

    const getGitHubAuthUrl = (): string => {
      const urlParams = new URLSearchParams(window.location.search);

      const originalUri = urlParams.get("original_uri");

      const state: string = (originalUri) ? originalUri : process.env.REACT_APP_COMPANY_WEBSITE!;

      const currentUrl = window.location.href.split('?')[0];

      return `https://github.com/login/oauth/authorize?client_id=${process.env.REACT_APP_GITHUB_CLIENT_ID}&redirect_uri=${currentUrl}&scope=user:email&state=${state}`;
    }

    const getUser = async (search: string) => {
      if (!user && !loggedIn) {
        const urlParams = new URLSearchParams(search);

        const code = urlParams.get("code");
        const redirectState = urlParams.get("state");
  
        const checkOAuthCode: Action = {
          method: 'GET',
          endpoint: `/oauth-callback?code=${code}&state=${redirectState}`
        }
  
        const res = await clientContext.query(checkOAuthCode);

        if (!res.error) {
          setUser(res.payload);
  
          setLoggedIn(true);
  
          setGHState(redirectState!);
  
          const user: GitHubUser = res.payload;

          setUsername({value: user.login, trimmedValue: user.login.trim(), isPristine: true});
          setEmail( (user.email) ? {value: user.email, trimmedValue: user.email.trim(), isPristine: true} : {value: "", trimmedValue: "", isPristine: true});
          setFullName( (user.name) ? {value: user.name, trimmedValue: user.name.trim(), isPristine: true} : {value: "", trimmedValue: "", isPristine: true});
        } else {
          setQueryError({error: true, errorMessage: res.payload});
        }
      }
    }

    const doSubmit = async (evt: FormEvent) => {
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
  
        const putSignCla: Action = {
          method: 'PUT',
          endpoint: '/sign-cla',
          body: signUser,
          headers: {
            Accept: 'application/json',
          },
        }
  
        const res = await clientContext.query(putSignCla);
  
        if (!res.error) {
          if (ghState !== "")
          window.location.href = decodeURI(ghState);
        } else {
          setQueryError({error: true, errorMessage: res.payload});
        }
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
          handleScroll={(e: any) =>
            handleScroll(e, setScrolled)
          }/>

        <NxCheckbox 
          checkboxId="sign-cla-check" 
          isChecked={agreeToTerms}
          disabled={true}>
          Sign the CLA version: {process.env.REACT_APP_CLA_VERSION}
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
                  <NxButton className={submitBtnClasses} variant="primary" type="submit">Sign the CLA</NxButton>
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
