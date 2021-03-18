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
import { NxButton, NxCheckbox, NxPageHeader } from "@sonatype/react-shared-components";
import CLABody from './ClaBody/CLABody';
import React from "react";

type CLAAppContainerState = {
  scrolled: boolean
}

const handleScroll = (event: any): boolean => {
  let el = event.target;
  if (Math.round(el.scrollTop + el.clientHeight) === el.scrollHeight) {
    return true;
  }
  return false;
};

class ClaAppContainer extends React.Component<any, CLAAppContainerState> {

  constructor(props: any) {
    super(props);

    this.state = {
      scrolled: false
    }
  }

  render() {
    return <React.Fragment>
    <div className="nx-page-header">

      <NxPageHeader 
        productInfo={
          { name: "THE CLA" }
        }/>

    </div>
    <div className="nx-page-content">

      <div className="nx-page-main">

        <h1>Sign the Sonatype Contributor License Agreement (CLA)</h1>

        <NxCheckbox 
          checkboxId="login-check" 
          isChecked={false}>
          Authenticate with GitHub so we can associate your commits with your signed CLA
        </NxCheckbox>

        <NxButton 
          variant="primary">
            Login To GitHub
        </NxButton>

        <NxCheckbox 
          checkboxId="cla-check" 
          isChecked={this.state.scrolled} 
          disabled={true}>
          Review the CLA
        </NxCheckbox>

        <CLABody 
          handleScroll={
            (e: any) => this.setState({scrolled: handleScroll(e)})
          }/>

        <NxCheckbox 
          checkboxId="login-check" 
          isChecked={false}>
          Sign the CLA
        </NxCheckbox>

        <NxButton 
          variant="primary">
          Login via GitHub to sign the CLA
        </NxButton>

      </div>

    </div>
    </React.Fragment>
  }
}

export default ClaAppContainer;

export { handleScroll };
