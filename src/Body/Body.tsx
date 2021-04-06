/*
 * Copyright (c) 2021-present Sonatype, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
import { NxCheckbox, NxFieldset, NxForm, NxFormGroup, NxTextInput } from "@sonatype/react-shared-components";
import React, { useContext, useState } from "react";
import { Action, ClientContext } from "react-fetching-library";
import CLABody from "../ClaBody/CLABody";

type GitHubUser = {
  login: string
  email?: string
}

type SignCla = {
  user: GitHubUser
  claVersion: string
}

const handleScroll = (event: any, setScrolled: (scrolled: boolean) => any) => {
  let el = event.target;
  if (Math.round(el.scrollTop + el.clientHeight) === el.scrollHeight) {
    setScrolled(true);
  }
};

const hasCode = (url: string): boolean => {
  return url.startsWith("?code=");
}

const githubAuthURL = `https://github.com/login/oauth/authorize?client_id=${process.env.REACT_APP_GITHUB_CLIENT_ID}&redirect_uri=${window.location.href}&scope=user:email&state=${window.location.href}`

const Body = () => {

    const [loggedIn, setLoggedIn] = useState(false);
    const [scrolled, setScrolled] = useState(false);
    const [username, setUsername] = useState<string>("");
    const [email, setEmail] = useState<string>("");
    const [user, setUser] = useState<GitHubUser | undefined>(undefined);
    const [agreeToTerms, setAgreeToTerms] = useState(false);

    const clientContext = useContext(ClientContext);

    const onEmailChange = (val: string) => {
      setEmail(val);
    }

    const getUser = async (search: string) => {
      if (!user && !loggedIn) {
        const urlParams = new URLSearchParams(search);

        const code = urlParams.get("code");
        const state = urlParams.get("state");
  
        const checkOAuthCode: Action = {
          method: 'GET',
          endpoint: `/oauth-callback?code=${code}&state=${state}`
        }
  
        const res = await clientContext.query(checkOAuthCode);
  
        setUser(res.payload);
  
        setLoggedIn(true);

        const user: GitHubUser = res.payload;

        setUsername(user.login);
        setEmail( (user.email) ? user.email : "");
      }
    }

    const doSubmit = async () => {
      const signUser: SignCla = { 
        user: { 
          login: user!.login, 
          email: email }, 
        claVersion: (process.env.REACT_APP_CLA_VERSION) ? process.env.REACT_APP_CLA_VERSION : ""
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

      console.log(res);
    }

    const doRender = (client: any) => {

      if (hasCode(window.location.search) && !loggedIn) {
        getUser(window.location.search);
      }

      return <div className="nx-page-content">

      <div className="nx-page-main">

        <h1>Sign the {process.env.REACT_APP_COMPANY_NAME} Contributor License Agreement (CLA)</h1>

        <NxCheckbox 
          checkboxId="login-check" 
          isChecked={loggedIn}
          disabled={true}>
          Authenticate with GitHub so we can associate your commits with your signed CLA
        </NxCheckbox>

        { !loggedIn && (
          <a href={githubAuthURL} className="nx-btn nx-btn--primary">Login to Github</a>
        )}

        { loggedIn && user && (
          <h3>Logged in as: { user.login }</h3>
        )}

        <NxCheckbox 
          checkboxId="cla-check" 
          isChecked={scrolled} 
          disabled={true}>
          Review the CLA
        </NxCheckbox>

        <CLABody 
          handleScroll={(e: any) =>
            handleScroll(e, setScrolled)
          }/>

        <NxCheckbox 
          checkboxId="login-check" 
          isChecked={agreeToTerms}
          disabled={true}>
          Sign the CLA
        </NxCheckbox>

        { !loggedIn && (
          <a href={githubAuthURL} className="nx-btn nx-btn--primary">Login via Github to sign the CLA</a>
        )}

        { loggedIn && user && (
          <NxForm 
            onSubmit={doSubmit}
            submitBtnText="Sign the CLA">

            <NxFormGroup 
              label="Username" 
              isRequired={true}>
              <NxTextInput 
                value={username} 
                isPristine={true} 
                disabled={true}
                required={true}/>
            </NxFormGroup>

            <NxFormGroup 
              label="Email Address" 
              isRequired={true}>
              <NxTextInput 
                value={email}
                onChange={onEmailChange} 
                isPristine={true}
                required={true}/>
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
          </NxForm>
        )}
        
      </div>

    </div>
    }

    return (
      doRender(clientContext)
    )
}

export default Body;

export { handleScroll, hasCode }
