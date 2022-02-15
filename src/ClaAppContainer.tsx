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
import Header from './Header/Header';
import Body from './Body/Body';
import React from "react";

class ClaAppContainer extends React.Component {

  render() {
    return <React.Fragment>
      <div className="nx-page-header">
        <Header />
      </div>
      <div className="nx-page-content">
        <div className="nx-page-main">
          <Body />
        </div>
      </div>
    </React.Fragment>
  }
}

export default ClaAppContainer;
