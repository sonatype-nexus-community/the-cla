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

import { handleScroll, hasCode } from './Body';

describe("<ClaAppContainer />", () => {
  test("Should be able to determine if a scroll has reached the bottom", async () => {
    const setScrolled = jest.fn();

    handleScroll(
      { target: { clientHeight: 420, scrollTop: 420, scrollHeight: 840 } },
      setScrolled
    )

    expect(setScrolled).toBeCalled()
    expect(setScrolled).toBeCalledWith(true);

    setScrolled.mockReset();

    handleScroll(
      {target: {clientHeight: 100, scrollTop: 420, scrollHeight: 840}},
      setScrolled
    )

    expect(setScrolled).toBeCalledTimes(0);
  });

  test("Should be able to determine if a code is in the URL", async () => {
    expect(hasCode("")).toBe(false);

    expect(hasCode("?code=thing")).toBe(true);
  });
});
