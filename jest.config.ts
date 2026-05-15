import type { Config } from 'jest';

const config: Config = {
  preset: 'ts-jest',
  testEnvironment: 'jsdom',
  setupFiles: ['<rootDir>/src/setupFetch.ts'],
  setupFilesAfterEnv: ['<rootDir>/src/setupTests.ts'],
  moduleNameMapper: {
    '\\.(css|scss)$': '<rootDir>/__mocks__/styleMock.ts',
    '\\.(svg|png|jpg|gif)$': '<rootDir>/__mocks__/fileMock.ts',
    '^@sonatype/react-shared-components$': '<rootDir>/__mocks__/@sonatype/react-shared-components.tsx',
    '^@sonatype/react-shared-components/(.*)$': '<rootDir>/__mocks__/@sonatype/react-shared-components-deep.ts',
  },
  testPathIgnorePatterns: ['/node_modules/', '/.claude/'],
  modulePathIgnorePatterns: ['/.claude/'],
  transform: {
    '^.+\\.(ts|tsx)$': ['ts-jest', { tsconfig: { jsx: 'react-jsx', strict: false } }],
  },
  transformIgnorePatterns: [
    'node_modules/(?!(@codemirror|@sonatype/react-shared-components))',
  ],
};

export default config;
