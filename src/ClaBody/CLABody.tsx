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
import React from 'react';
import { NxLoadingSpinner } from '@sonatype/react-shared-components';
import { Action, useQuery } from 'react-fetching-library';

const fetchCLAText: Action = {
  method: 'GET',
  endpoint: '/cla-text'
};

type CLABodyProps = {
  handleScroll: (event: any) => void;
}

const CLABody = (props: CLABodyProps) => {

  const { loading, payload, error, errorObject } = useQuery(fetchCLAText);

  if (error) {
    console.log("errorObject: " + errorObject)
    return (
      <h1 data-testid="cla-body-error">There was an error!</h1>
    )
  }

  if (loading) {
    return (
      <NxLoadingSpinner />
    )
  }

  if (payload) {
    return (
      <React.Fragment>
        <pre className="nx-pre nx-scrollable" onScroll={props.handleScroll}>
          {payload}
        </pre>
      </React.Fragment>
    )
  }

  return null;
}

export default CLABody;
