# The CSS

Just like with [downloading the Javascript](javascript), you can do with the CSS.

[**https://raw.githubusercontent.com/peterbe/autocompeter/master/public/dist/autocompeter-v1.min.css**](https://raw.githubusercontent.com/peterbe/autocompeter/master/public/dist/autocompeter-v1.min.css)

Or...

    bower install autocompeter
    ls bower_components/autocompeter/public/dist/*.css

Or...

    <link rel="stylesheet" href="//cdn.autocompeter.com/dist/autocompeter-v1.min.css">

There is also another alternative. If you already use [Sass (aka. SCSS)](http://sass-lang.com/)
you can download [autocompeter.scss](https://github.com/peterbe/autocompeter/blob/master/src/autocompeter.scss)
instead and incorporate that into your own build system.

## Overriding

It's very possible that on your site, the CSS doesn't fit in perfectly. Either
you don't exactly like the way it looks or it just doesn't work as expected.
The recommended way to deal with this is to override certain selectors. For
example it might look like this:

    <link rel="stylesheet" href="//cdn.autocompeter.com/dist/autocompeter-v1.min.css">
    <style>
    ._ac-wrap { width: 400px; }
    @media only screen and (max-width : 321px) {
      ._ac-wrap { width: 290px; }
    }
    </style>

As an example, with the design being used on
[autocompeter.com](http://autocompeter.com) some CSS had to be overridden.
