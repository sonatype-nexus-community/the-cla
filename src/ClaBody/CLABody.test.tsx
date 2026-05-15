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
import { render, act } from '@testing-library/react';
import React from 'react';

import CLABody from './CLABody';

describe("<CLABody />", () => {
  test("Should display an error by default", async () => {
    // fetch is not available in jsdom; the promise rejects, ErrorBoundary catches it
    let container: ReturnType<typeof render>;

    await act(async () => {
      container = render(
        <React.Suspense fallback={null}>
          <CLABody handleScroll={() => {}}/>
        </React.Suspense>
      );
    });

    const h1 = await container!.findByTestId(`cla-body-error`);

    expect(h1).toHaveTextContent("There was an error!");
  });
});
