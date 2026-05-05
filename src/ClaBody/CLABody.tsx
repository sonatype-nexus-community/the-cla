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
import React, { use, Suspense } from 'react';
import { NxLoadingSpinner } from '@sonatype/react-shared-components';
import ErrorBoundary from '../ErrorBoundary';

// Promise created at module scope to avoid infinite render loop.
// The no-op .catch suppresses unhandled-rejection warnings in Node/test environments;
// React's use() hook re-throws for ErrorBoundary when the component renders.
const claTextPromise: Promise<string> = fetch('/cla-text').then(res => {
  if (!res.ok) throw new Error(`Failed to fetch CLA text: ${res.status}`);
  return res.text();
});
// Prevent unhandled rejection in Node; use() will surface the error to ErrorBoundary
claTextPromise.catch(() => {});

type CLABodyProps = {
  handleScroll: (event: React.UIEvent<HTMLPreElement>) => void;
}

const CLABodyInner = (props: CLABodyProps) => {
  const claText = use(claTextPromise);

  return (
    <React.Fragment>
      <pre className="nx-pre nx-scrollable" onScroll={props.handleScroll}>
        {claText}
      </pre>
    </React.Fragment>
  );
};

const CLABody = (props: CLABodyProps) => {
  return (
    <ErrorBoundary>
      <Suspense fallback={<NxLoadingSpinner />}>
        <CLABodyInner handleScroll={props.handleScroll} />
      </Suspense>
    </ErrorBoundary>
  );
};

export default CLABody;
