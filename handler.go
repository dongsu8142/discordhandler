package discordhandler

import (
	"fmt"
	"log"
	"regexp"
	"strings"

	"github.com/bwmarrin/discordgo"
)

type Route struct {
	Pattern     string
	Description string
	Help        string
	Run         HandlerFunc
}

type Context struct {
	Fields          []string
	Content         string
	IsDirected      bool
	IsPrivate       bool
	HasPrefix       bool
	HasMention      bool
	HasMentionFirst bool
}

type HandlerFunc func(*Handler, *discordgo.Session, *discordgo.Message, *Context)

type Handler struct {
	Routes  []*Route
	Default *Route
	Prefix  string
}

func New(prefix string) *Handler {
	m := &Handler{}
	m.Prefix = prefix
	return m
}

func (h *Handler) Route(pattern, desc string, cb HandlerFunc) (*Route, error) {

	r := Route{}
	r.Pattern = pattern
	r.Description = desc
	r.Run = cb
	h.Routes = append(h.Routes, &r)

	return &r, nil
}

func (h *Handler) FuzzyMatch(msg string) (*Route, []string) {
	fields := strings.Fields(msg)
	if len(fields) == 0 {
		return nil, nil
	}
	var r *Route
	var rank int
	var fk int
	for fk, fv := range fields {
		for _, rv := range h.Routes {
			if rv.Pattern == fv {
				return rv, fields[fk:]
			}
			if strings.HasPrefix(rv.Pattern, fv) {
				if len(fv) > rank {
					r = rv
					rank = len(fv)
				}
			}
		}
	}
	return r, fields[fk:]
}

func (h *Handler) OnMessageCreate(ds *discordgo.Session, mc *discordgo.MessageCreate) {
	var err error
	if mc.Author.ID == ds.State.User.ID {
		return
	}
	ctx := &Context{
		Content: strings.TrimSpace(mc.Content),
	}

	var c *discordgo.Channel
	c, err = ds.State.Channel(mc.ChannelID)
	if err != nil {
		c, err = ds.Channel(mc.ChannelID)
		if err != nil {
			log.Printf("unable to fetch Channel for Message, %s", err)
		} else {
			err = ds.State.ChannelAdd(c)
			if err != nil {
				log.Printf("error updating State with Channel, %s", err)
			}
		}
	}
	if c != nil {
		if c.Type == discordgo.ChannelTypeDM {
			ctx.IsPrivate, ctx.IsDirected = true, true
		}
	}
	if !ctx.IsDirected {
		for _, v := range mc.Mentions {
			if v.ID == ds.State.User.ID {
				ctx.IsDirected, ctx.HasMention = true, true
				reg := regexp.MustCompile(fmt.Sprintf("<@!?(%s)>", ds.State.User.ID))
				if reg.FindStringIndex(ctx.Content)[0] == 0 {
					ctx.HasMentionFirst = true
				}
				ctx.Content = reg.ReplaceAllString(ctx.Content, "")

				break
			}
		}
	}
	if !ctx.IsDirected && len(h.Prefix) > 0 {
		if strings.HasPrefix(ctx.Content, h.Prefix) {
			ctx.IsDirected, ctx.HasPrefix, ctx.HasMentionFirst = true, true, true
			ctx.Content = strings.TrimPrefix(ctx.Content, h.Prefix)
		}
	}
	if !ctx.IsDirected {
		return
	}
	r, fl := h.FuzzyMatch(ctx.Content)
	if r != nil {
		ctx.Fields = fl
		r.Run(h, ds, mc.Message, ctx)
		return
	}
	if h.Default != nil && (ctx.HasMentionFirst) {
		h.Default.Run(h, ds, mc.Message, ctx)
	}
}
