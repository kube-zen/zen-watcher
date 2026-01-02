package commands

import "context"

type Options struct {
	Kubeconfig   string
	Context      string
	Namespace    string
	AllNamespaces bool
}

type optionsKey struct{}

func WithOptions(ctx context.Context, opts Options) context.Context {
	return context.WithValue(ctx, optionsKey{}, opts)
}

func OptionsFromContext(ctx context.Context) Options {
	if opts, ok := ctx.Value(optionsKey{}).(Options); ok {
		return opts
	}
	return Options{}
}

