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

import path from 'node:path';
import { fileURLToPath } from 'node:url';
import webpack from 'webpack';
import CopyWebpackPlugin from 'copy-webpack-plugin';
import HtmlWebpackPlugin from 'html-webpack-plugin';
import MiniCssExtractPlugin from 'mini-css-extract-plugin';
import type { Configuration as DevServerConfiguration } from 'webpack-dev-server';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

type WebpackEnv = Record<string, string | boolean>;
type WebpackArgv = { mode?: 'development' | 'production' | 'none' };

const config = (env: WebpackEnv, argv: WebpackArgv): webpack.Configuration & { devServer?: DevServerConfiguration } => {
  const isDevelopment = argv.mode === 'development';

  return {
    entry: './src/index.tsx',

    output: {
      path: path.resolve(__dirname, 'build'),
      filename: 'static/js/[name].[contenthash].js',
      clean: true,
      publicPath: '/',
    },

    resolve: {
      extensions: ['.ts', '.tsx', '.js', '.jsx'],
    },

    module: {
      rules: [
        {
          test: /\.(ts|tsx)$/,
          use: {
            loader: 'ts-loader',
            options: {
              compilerOptions: {
                noEmit: false,
              },
            },
          },
          exclude: /node_modules/,
        },
        {
          test: /\.module\.(scss|css)$/,
          use: [
            MiniCssExtractPlugin.loader,
            { loader: 'css-loader', options: { esModule: false, modules: { localIdentName: '[local]' } } },
            'sass-loader',
          ],
        },
        {
          test: /(?<!\.module)\.(scss|css)$/,
          use: [
            MiniCssExtractPlugin.loader,
            'css-loader',
            'sass-loader',
          ],
        },
        {
          test: /\.(svg|png|jpg|jpeg|gif|ico)$/i,
          type: 'asset/resource',
        },
      ],
    },

    plugins: [
      new CopyWebpackPlugin({
        patterns: [
          { from: 'public', to: '.', globOptions: { ignore: ['**/index.html'] } },
        ],
      }),
      new HtmlWebpackPlugin({
        template: './public/index.html',
      }),
      new MiniCssExtractPlugin({
        filename: 'static/css/[name].[contenthash].css',
      }),
      new webpack.DefinePlugin({
        'process.env.REACT_APP_CLA_URL': JSON.stringify(process.env.REACT_APP_CLA_URL ?? ''),
        'process.env.REACT_APP_COMPANY_NAME': JSON.stringify(process.env.REACT_APP_COMPANY_NAME ?? ''),
        'process.env.REACT_APP_COMPANY_WEBSITE': JSON.stringify(process.env.REACT_APP_COMPANY_WEBSITE ?? ''),
        'process.env.REACT_APP_CLA_APP_NAME': JSON.stringify(process.env.REACT_APP_CLA_APP_NAME ?? ''),
        'process.env.REACT_APP_CLA_VERSION': JSON.stringify(process.env.REACT_APP_CLA_VERSION ?? ''),
        'process.env.REACT_APP_GITHUB_CLIENT_ID': JSON.stringify(process.env.REACT_APP_GITHUB_CLIENT_ID ?? ''),
      }),
    ],

    devtool: isDevelopment ? 'source-map' : false,

    devServer: {
      port: 3000,
      historyApiFallback: true,
      proxy: [
        {
          context: ['/cla-text', '/oauth-callback', '/sign-cla', '/webhook-integration', '/info', '/build-info'],
          target: 'http://localhost:4200',
        },
      ],
    },
  };
};

export default config;
